package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

func TestVersionOneSettingsMigrateToCurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "livecheck.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE app_settings (id INTEGER PRIMARY KEY CHECK (id = 1), revision INTEGER NOT NULL, settings_json TEXT NOT NULL, updated_at TEXT NOT NULL);
INSERT INTO schema_migrations VALUES (1, '2026-11-01T00:00:00Z');
INSERT INTO app_settings VALUES (1, 4, '{"microphone":{"name":"Old mic"},"stream_track":2}', '2026-11-01T00:00:00Z');`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	st, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("migrate v1: %v", err)
	}
	defer st.Close()
	version, _ := st.SchemaVersion(context.Background())
	if version != 2 {
		t.Fatalf("version = %d, want 2", version)
	}
	settings, err := st.LoadSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if settings.Revision != 4 || settings.Microphone == nil || settings.Microphone.Name != "Old mic" || settings.StreamTrack == nil || *settings.StreamTrack != 2 {
		t.Fatalf("v1 settings not preserved: %+v", settings)
	}
}

func TestIncidentReportDedupGroupingAndFeedback(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := st.StartMonitoringSession(ctx, "session-1", now); err != nil {
		t.Fatal(err)
	}
	id, publicID := NewIncidentIDs()
	inc, accepted, err := st.AcceptReport(ctx, "session-1", "audio", "delivery-1", "message-1", "viewer-1", id, publicID, now)
	if err != nil || !accepted {
		t.Fatalf("first report: %+v accepted=%v err=%v", inc, accepted, err)
	}
	if inc.ReportCount != 1 || inc.DistinctReporters != 1 {
		t.Fatalf("first counts = %+v", inc)
	}
	id2, public2 := NewIncidentIDs()
	_, accepted, err = st.AcceptReport(ctx, "session-1", "audio", "delivery-1", "message-2", "viewer-1", id2, public2, now)
	if err != nil || accepted {
		t.Fatalf("same delivery counted: accepted=%v err=%v", accepted, err)
	}
	_, accepted, err = st.AcceptReport(ctx, "session-1", "audio", "delivery-2", "message-1", "viewer-1", id2, public2, now)
	if err != nil || accepted {
		t.Fatalf("same message counted: accepted=%v err=%v", accepted, err)
	}
	inc, accepted, err = st.AcceptReport(ctx, "session-1", "audio", "delivery-3", "message-3", "viewer-2", id2, public2, now)
	if err != nil || !accepted {
		t.Fatalf("second reporter: accepted=%v err=%v", accepted, err)
	}
	if inc.ID != id || inc.ReportCount != 2 || inc.DistinctReporters != 2 {
		t.Fatalf("grouping/counts = %+v", inc)
	}
	inc.Stage = domain.StageAwaitingFeedback
	inc.Findings = []domain.Finding{{ID: "f1", IncidentID: inc.ID, Code: domain.FindingMICMuted, Category: "audio"}}
	if err := st.SaveIncident(ctx, inc); err != nil {
		t.Fatal(err)
	}
	inc, err = st.AddFeedback(ctx, id, "viewer-1", "negative", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if inc.Feedback.Negative != 1 || inc.Stage != domain.StageUnresolved {
		t.Fatalf("negative feedback = %+v", inc)
	}
	inc, _ = st.AddFeedback(ctx, id, "viewer-1", "positive", now.Add(2*time.Second))
	if inc.Feedback.Positive != 1 || inc.Feedback.Negative != 0 {
		t.Fatalf("latest vote did not replace: %+v", inc.Feedback)
	}
}

func TestIncidentDismissAndRestartInterrupt(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	id, publicID := NewIncidentIDs()
	inc, err := st.CreateManualIncident(ctx, "manual-1", "audio", id, publicID, now)
	if err != nil {
		t.Fatal(err)
	}
	inc.Stage = domain.StageInvestigating
	if err := st.SaveIncident(ctx, inc); err != nil {
		t.Fatal(err)
	}
	if err := st.InterruptOpenIncidents(ctx, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	loaded, _ := st.GetIncident(ctx, id)
	if loaded.Stage != domain.StageInterrupted {
		t.Fatalf("restart stage = %s", loaded.Stage)
	}
	dismissed, err := st.DismissIncident(ctx, id, now.Add(2*time.Second))
	if err != nil || dismissed.Stage != domain.StageDismissed {
		t.Fatalf("dismiss = %+v, %v", dismissed, err)
	}
}

func TestDurableDedupSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dedup.db")
	ctx := context.Background()
	now := time.Now().UTC()
	st, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.StartMonitoringSession(ctx, "session-1", now); err != nil {
		t.Fatal(err)
	}
	id, publicID := NewIncidentIDs()
	if _, accepted, err := st.AcceptReport(ctx, "session-1", "audio", "delivery-1", "message-1", "viewer-1", id, publicID, now); err != nil || !accepted {
		t.Fatalf("first report: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	id2, public2 := NewIncidentIDs()
	_, accepted, err := reopened.AcceptReport(ctx, "session-1", "audio", "delivery-1", "message-9", "viewer-1", id2, public2, now.Add(time.Second))
	if err != nil || accepted {
		t.Fatalf("reopened duplicate accepted=%v err=%v", accepted, err)
	}
}
