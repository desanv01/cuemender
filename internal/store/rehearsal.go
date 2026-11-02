package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

// ReplaceRehearsal commits the complete idle scenario boundary. The coordinator
// owns admission; no provider or cache changes occur before this transaction.
func (s *Store) ReplaceRehearsal(ctx context.Context, revision int64, exp domain.Expectations, sessionID string, incidents []domain.Incident, proposals []domain.Proposal, now time.Time) (domain.Settings, error) {
	if s.mode != domain.ModeRehearsal {
		return domain.Settings{}, domain.ErrModeConflict
	}
	if err := exp.Validate(); err != nil {
		return domain.Settings{}, err
	}
	raw, err := json.Marshal(exp)
	if err != nil {
		return domain.Settings{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Settings{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE app_settings SET revision=revision+1, settings_json=?, updated_at=? WHERE id=1 AND revision=?`, string(raw), formatTime(now), revision)
	if err != nil {
		return domain.Settings{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.Settings{}, err
	}
	if n != 1 {
		return domain.Settings{}, domain.ErrRevisionConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE monitoring_sessions SET active=0, stopped_at=? WHERE id=? AND active=1`, formatTime(now), sessionID); err != nil {
		return domain.Settings{}, err
	}
	for _, inc := range incidents {
		if err = saveIncidentTx(ctx, tx, inc); err != nil {
			return domain.Settings{}, err
		}
	}
	for _, p := range proposals {
		encoded, err := json.Marshal(p)
		if err != nil {
			return domain.Settings{}, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE proposals SET status=?,proposal_json=? WHERE id=?`, p.Status, string(encoded), p.ID); err != nil {
			return domain.Settings{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return domain.Settings{}, err
	}
	return domain.Settings{Revision: revision + 1, Expectations: exp}, nil
}
