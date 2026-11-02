// Promoted from Codex's P03-R02 review set; assertions preserved.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/desanv01/cuemender/internal/domain"
	"testing"
	"time"
)

func TestReviewR01DedupFractionalExpiry(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	if _, ok, err := st.AcceptReport(ctx, "s", "audio", "d", "m", "a", "i", "CM-ONE", now); err != nil || !ok {
		t.Fatalf("seed: %v %v", ok, err)
	}
	if _, ok, err := st.AcceptReport(ctx, "s", "audio", "d", "m", "a", "j", "CM-TWO", now.Add(24*time.Hour+500*time.Millisecond)); err != nil || !ok {
		t.Fatalf("expired ID retained across fractional-second boundary: accepted=%v err=%v", ok, err)
	}
}

func TestReviewR01ExactReporterFeedbackBounds(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	inc, ok, err := st.AcceptReport(ctx, "s", "audio", "seed-d", "seed-m", "a0", "inc", "CM-BOUND", now)
	if err != nil || !ok {
		t.Fatalf("seed: %v %v", ok, err)
	}
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < 10000; i++ {
		actor := fmt.Sprintf("a%d", i)
		if i > 0 {
			if _, err := tx.Exec(`INSERT INTO reports(incident_id,session_id,actor_id,received_at) VALUES(?,?,?,?)`, inc.ID, "s", actor, formatTime(now)); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO feedback(incident_id,actor_id,vote,updated_at) VALUES(?,?,?,?)`, inc.ID, actor, "positive", formatTime(now)); err != nil {
			t.Fatal(err)
		}
	}
	inc.ReportCount = 10000
	inc.DistinctReporters = 10000
	inc.Feedback.Positive = 10000
	inc.Stage = domain.StageAwaitingFeedback
	raw, _ := json.Marshal(inc)
	if _, err := tx.Exec(`UPDATE incidents SET report_count=10000,distinct_reporters=10000,feedback_positive=10000,stage=?,incident_json=? WHERE id=?`, inc.Stage, string(raw), inc.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := st.AcceptReport(ctx, "s", "audio", "new-d", "new-m", "new-actor", "unused", "CM-NEW", now); ok || !errors.Is(err, ErrReportBound) {
		t.Fatalf("report bound accepted=%v err=%v", ok, err)
	}
	got, ok, err := st.AcceptReport(ctx, "s", "audio", "existing-d", "existing-m", "a0", "unused", "CM-NEW", now)
	if err != nil || !ok || got.ReportCount != 10001 || got.DistinctReporters != 10000 {
		t.Fatalf("existing actor counts: accepted=%v count=%d distinct=%d err=%v", ok, got.ReportCount, got.DistinctReporters, err)
	}
	if _, err := st.AddFeedback(ctx, inc.ID, "new-actor", "negative", now); !errors.Is(err, ErrReportBound) {
		t.Fatalf("feedback bound: %v", err)
	}
	got, err = st.AddFeedback(ctx, inc.ID, "a0", "negative", now)
	if err != nil || got.Feedback.Positive != 9999 || got.Feedback.Negative != 1 {
		t.Fatalf("vote replacement: %+v %v", got.Feedback, err)
	}
	for _, table := range []string{"reports", "feedback"} {
		var n int
		if err := st.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil || n != 10000 {
			t.Fatalf("%s count=%d err=%v", table, n, err)
		}
	}
	var leaked int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM report_dedup WHERE key IN ('d:new-d','m:new-m')`).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("rejected report left dedup=%d err=%v", leaked, err)
	}
}
