package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

var (
	ErrIncidentNotFound = errors.New("incident not found")
	ErrReportBound      = errors.New("report detail bound reached")
)

func randomID(prefix string, n int) string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	out := make([]byte, n)
	for i := range buf {
		out[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return prefix + string(out)
}

func NewIncidentIDs() (id, publicID string) {
	return randomID("inc-", 20), randomID("CM-", 6)
}

func NewSessionID() string { return randomID("sess-", 20) }

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func (s *Store) StartMonitoringSession(ctx context.Context, id string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE monitoring_sessions SET active = 0, stopped_at = ? WHERE active = 1`, formatTime(now)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO monitoring_sessions (id, active, started_at) VALUES (?, 1, ?)`, id, formatTime(now)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) StopMonitoringSession(ctx context.Context, sessionID string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE monitoring_sessions SET active = 0, stopped_at = ? WHERE id = ? AND active = 1`, formatTime(now), sessionID)
	return err
}

func (s *Store) StopAllMonitoringSessions(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE monitoring_sessions SET active = 0, stopped_at = ? WHERE active = 1`, formatTime(now))
	return err
}

func (s *Store) ActiveMonitoringSession(ctx context.Context) (*domain.MonitoringSessionInfo, error) {
	var id, started string
	err := s.db.QueryRowContext(ctx, `SELECT id, started_at FROM monitoring_sessions WHERE active = 1 ORDER BY started_at DESC LIMIT 1`).Scan(&id, &started)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &domain.MonitoringSessionInfo{ID: id, Active: true, StartedAt: parseTime(started)}, nil
}

func incidentFromRow(scan func(...any) error) (domain.Incident, error) {
	var raw string
	if err := scan(&raw); err != nil {
		return domain.Incident{}, err
	}
	var inc domain.Incident
	if err := json.Unmarshal([]byte(raw), &inc); err != nil {
		return domain.Incident{}, err
	}
	return inc, nil
}

const incidentSelect = `SELECT incident_json FROM incidents`

func (s *Store) GetIncident(ctx context.Context, id string) (domain.Incident, error) {
	return incidentFromRow(func(dest ...any) error {
		return s.db.QueryRowContext(ctx, incidentSelect+` WHERE id = ?`, id).Scan(dest...)
	})
}

func (s *Store) PublicIncidentByID(ctx context.Context, publicID string) (domain.Incident, error) {
	return incidentFromRow(func(dest ...any) error {
		return s.db.QueryRowContext(ctx, incidentSelect+` WHERE public_id = ?`, publicID).Scan(dest...)
	})
}

func (s *Store) ActiveIncident(ctx context.Context, sessionID, category string) (*domain.Incident, error) {
	inc, err := incidentFromRow(func(dest ...any) error {
		return s.db.QueryRowContext(ctx, incidentSelect+` WHERE session_id = ? AND category = ? AND stage NOT IN (?,?,?,?) ORDER BY julianday(updated_at) DESC LIMIT 1`,
			sessionID, category, domain.StageDismissed, domain.StageClosed, domain.StageExpired, domain.StageInvalidated).Scan(dest...)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inc, nil
}

func (s *Store) ListIncidents(ctx context.Context, limit int) ([]domain.Incident, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx, incidentSelect+` ORDER BY julianday(updated_at) DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Incident, 0)
	for rows.Next() {
		inc, err := incidentFromRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	return out, rows.Err()
}

// AcceptReport atomically enforces durable delivery/message dedup, groups the
// report into one active category incident, and records bounded reporter detail.
func (s *Store) AcceptReport(ctx context.Context, sessionID, category, deliveryID, messageID, actorID string, incidentID, publicID string, now time.Time) (domain.Incident, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Incident{}, false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM report_dedup WHERE julianday(retained_until) <= julianday(?)`, formatTime(now)); err != nil {
		return domain.Incident{}, false, err
	}
	for _, key := range []string{"d:" + deliveryID, "m:" + messageID} {
		if key == "d:" || key == "m:" {
			continue
		}
		res, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO report_dedup (key, retained_until) VALUES (?, ?)`, key, formatTime(now.Add(24*time.Hour)))
		if err != nil {
			return domain.Incident{}, false, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return domain.Incident{}, false, nil
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM report_dedup WHERE key NOT IN (SELECT key FROM report_dedup ORDER BY julianday(retained_until) DESC, key ASC LIMIT 10000)`); err != nil {
		return domain.Incident{}, false, err
	}

	var existingID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM incidents WHERE session_id = ? AND category = ? AND stage NOT IN (?,?,?,?) ORDER BY julianday(updated_at) DESC LIMIT 1`,
		sessionID, category, domain.StageDismissed, domain.StageClosed, domain.StageExpired, domain.StageInvalidated).Scan(&existingID)
	var inc domain.Incident
	if errors.Is(err, sql.ErrNoRows) {
		inc = domain.Incident{
			ID: incidentID, PublicID: publicID, SessionID: sessionID, Mode: domain.ModeLive,
			Origin: domain.OriginViewer, Category: category, Stage: domain.StageReported,
			OpenedAt: now, UpdatedAt: now, LocalVerification: domain.LocalNotAttempted,
			Findings: []domain.Finding{}, Checks: []domain.DiagnosticCheck{},
		}
	} else if err != nil {
		return domain.Incident{}, false, err
	} else {
		inc, err = incidentFromRow(func(dest ...any) error {
			return tx.QueryRowContext(ctx, incidentSelect+` WHERE id = ?`, existingID).Scan(dest...)
		})
		if err != nil {
			return domain.Incident{}, false, err
		}
	}
	var detailCount int
	if existingID == "" {
		raw, _ := json.Marshal(inc)
		if _, err := tx.ExecContext(ctx, `INSERT INTO incidents (id, public_id, session_id, mode, origin, category, stage, opened_at, updated_at, report_count, distinct_reporters, local_verification, feedback_positive, feedback_negative, incident_json) VALUES (?,?,?,?,?,?,?,?,?,0,0,?,0,0,?)`,
			inc.ID, inc.PublicID, inc.SessionID, inc.Mode, inc.Origin, inc.Category, inc.Stage, formatTime(inc.OpenedAt), formatTime(inc.UpdatedAt), inc.LocalVerification, string(raw)); err != nil {
			return domain.Incident{}, false, err
		}
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM reports WHERE incident_id = ?`, inc.ID).Scan(&detailCount); err != nil {
		return domain.Incident{}, false, err
	}
	increaseDistinct := true
	if detailCount >= 10000 {
		var actorExists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM reports WHERE incident_id = ? AND actor_id = ?`, inc.ID, actorID).Scan(&actorExists); err != nil {
			return domain.Incident{}, false, err
		}
		if actorExists == 0 {
			return domain.Incident{}, false, ErrReportBound
		}
	}
	res, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO reports (incident_id, session_id, delivery_id, message_id, actor_id, received_at) VALUES (?,?,?,?,?,?)`,
		inc.ID, sessionID, deliveryID, messageID, actorID, formatTime(now))
	if err != nil {
		return domain.Incident{}, false, err
	}
	if n, _ := res.RowsAffected(); n == 1 {
		inc.DistinctReporters++
	} else {
		increaseDistinct = false
	}
	_ = increaseDistinct
	inc.ReportCount++
	inc.UpdatedAt = now
	raw, _ := json.Marshal(inc)
	_, err = tx.ExecContext(ctx, `UPDATE incidents SET updated_at = ?, report_count = ?, distinct_reporters = ?, incident_json = ? WHERE id = ?`,
		formatTime(inc.UpdatedAt), inc.ReportCount, inc.DistinctReporters, string(raw), inc.ID)
	if err != nil {
		return domain.Incident{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Incident{}, false, err
	}
	return inc, true, nil
}

func (s *Store) CreateManualIncident(ctx context.Context, sessionID, category, incidentID, publicID string, now time.Time) (domain.Incident, error) {
	inc := domain.Incident{
		ID: incidentID, PublicID: publicID, SessionID: sessionID, Mode: domain.ModeLive,
		Origin: domain.OriginManual, Category: category, Stage: domain.StageReported,
		OpenedAt: now, UpdatedAt: now, LocalVerification: domain.LocalNotAttempted,
		Findings: []domain.Finding{}, Checks: []domain.DiagnosticCheck{},
	}
	raw, _ := json.Marshal(inc)
	_, err := s.db.ExecContext(ctx, `INSERT INTO incidents (id, public_id, session_id, mode, origin, category, stage, opened_at, updated_at, report_count, distinct_reporters, local_verification, feedback_positive, feedback_negative, incident_json) VALUES (?,?,?,?,?,?,?,?,?,0,0,?,0,0,?)`,
		inc.ID, inc.PublicID, inc.SessionID, inc.Mode, inc.Origin, inc.Category, inc.Stage, formatTime(inc.OpenedAt), formatTime(inc.UpdatedAt), inc.LocalVerification, string(raw))
	return inc, err
}

func (s *Store) SaveIncident(ctx context.Context, inc domain.Incident) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Report joins and feedback can advance indexed counters independently of a
	// stale in-memory finding snapshot. Merge them before replacing JSON.
	var reportCount, distinct, positive, negative int
	if err := tx.QueryRowContext(ctx, `SELECT report_count, distinct_reporters, feedback_positive, feedback_negative FROM incidents WHERE id = ?`, inc.ID).Scan(&reportCount, &distinct, &positive, &negative); err != nil {
		return err
	}
	inc.ReportCount, inc.DistinctReporters = reportCount, distinct
	inc.Feedback.Positive, inc.Feedback.Negative = positive, negative
	raw, _ := json.Marshal(inc)
	if _, err := tx.ExecContext(ctx, `UPDATE incidents SET stage = ?, updated_at = ?, local_verification = ?, feedback_positive = ?, feedback_negative = ?, closed_at = ?, evidence_at = ?, incident_json = ? WHERE id = ?`,
		inc.Stage, formatTime(inc.UpdatedAt), inc.LocalVerification, inc.Feedback.Positive, inc.Feedback.Negative,
		nullableTime(inc.ClosedAt), nullableTime(inc.EvidenceAt), string(raw), inc.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM findings WHERE incident_id = ?`, inc.ID); err != nil {
		return err
	}
	for _, finding := range inc.Findings {
		fraw, _ := json.Marshal(finding)
		if _, err := tx.ExecContext(ctx, `INSERT INTO findings (id, incident_id, code, category, finding_json) VALUES (?,?,?,?,?)`,
			finding.ID, inc.ID, finding.Code, finding.Category, string(fraw)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

func (s *Store) DismissIncident(ctx context.Context, incidentID string, now time.Time) (domain.Incident, error) {
	inc, err := s.GetIncident(ctx, incidentID)
	if err != nil {
		return domain.Incident{}, err
	}
	if domain.StageTerminal(inc.Stage) {
		return inc, nil
	}
	inc.Stage = domain.StageDismissed
	inc.UpdatedAt = now
	inc.ClosedAt = &now
	if err := s.SaveIncident(ctx, inc); err != nil {
		return domain.Incident{}, err
	}
	return inc, nil
}

func (s *Store) AddFeedback(ctx context.Context, incidentID, actorID, vote string, now time.Time) (domain.Incident, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Incident{}, err
	}
	defer tx.Rollback()
	inc, err := incidentFromRow(func(dest ...any) error {
		return tx.QueryRowContext(ctx, incidentSelect+` WHERE id = ?`, incidentID).Scan(dest...)
	})
	if err != nil {
		return domain.Incident{}, err
	}
	if domain.StageTerminal(inc.Stage) {
		return inc, nil
	}
	var actorCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM feedback WHERE incident_id = ?`, incidentID).Scan(&actorCount); err != nil {
		return domain.Incident{}, err
	}
	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM feedback WHERE incident_id = ? AND actor_id = ?`, incidentID, actorID).Scan(&existing); err != nil {
		return domain.Incident{}, err
	}
	if actorCount >= 10000 && existing == 0 {
		return domain.Incident{}, ErrReportBound
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO feedback (incident_id, actor_id, vote, updated_at) VALUES (?,?,?,?) ON CONFLICT(incident_id, actor_id) DO UPDATE SET vote = excluded.vote, updated_at = excluded.updated_at`,
		incidentID, actorID, vote, formatTime(now)); err != nil {
		return domain.Incident{}, err
	}
	var positive, negative int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN vote = 'positive' THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN vote = 'negative' THEN 1 ELSE 0 END),0) FROM feedback WHERE incident_id = ?`, incidentID).Scan(&positive, &negative); err != nil {
		return domain.Incident{}, err
	}
	inc.Feedback.Positive, inc.Feedback.Negative = positive, negative
	inc.UpdatedAt = now
	if negative > 0 {
		inc.Stage = domain.StageUnresolved
	}
	raw, _ := json.Marshal(inc)
	if _, err := tx.ExecContext(ctx, `UPDATE incidents SET stage = ?, updated_at = ?, feedback_positive = ?, feedback_negative = ?, incident_json = ? WHERE id = ?`,
		inc.Stage, formatTime(inc.UpdatedAt), inc.Feedback.Positive, inc.Feedback.Negative, string(raw), inc.ID); err != nil {
		return domain.Incident{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Incident{}, err
	}
	return inc, nil
}

// InterruptOpenIncidents closes prior-session work at startup. It never
// auto-monitors or replays work.
func (s *Store) InterruptOpenIncidents(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT incident_json FROM incidents WHERE stage NOT IN (?,?,?,?)`, domain.StageDismissed, domain.StageClosed, domain.StageExpired, domain.StageInvalidated)
	if err != nil {
		return err
	}
	var incidents []domain.Incident
	for rows.Next() {
		inc, err := incidentFromRow(rows.Scan)
		if err != nil {
			rows.Close()
			return err
		}
		incidents = append(incidents, inc)
	}
	rows.Close()
	for _, inc := range incidents {
		inc.Stage = domain.StageInterrupted
		inc.UpdatedAt = now
		if err := s.SaveIncident(ctx, inc); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) PurgeCompletedIncidents(ctx context.Context, now time.Time) error {
	cutoff := formatTime(now.Add(-30 * 24 * time.Hour))
	if _, err := s.db.ExecContext(ctx, `DELETE FROM incidents WHERE stage IN (?,?,?,?) AND julianday(updated_at) < julianday(?)`,
		domain.StageDismissed, domain.StageClosed, domain.StageExpired, domain.StageInvalidated, cutoff); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM incidents WHERE id IN (SELECT id FROM incidents WHERE stage IN (?,?,?,?) ORDER BY julianday(updated_at) DESC LIMIT -1 OFFSET 500)`,
		domain.StageDismissed, domain.StageClosed, domain.StageExpired, domain.StageInvalidated)
	return err
}
