//go:build reviewrepro

package store

import (
	"context"
	"fmt"
	"github.com/desanv01/cuemender/internal/domain"
	"testing"
	"time"
)

func TestReviewP03DedupExpiresAt24Hours(t *testing.T) {
	st := openTestStore(t)
	now := time.Now().UTC()
	ctx := context.Background()
	if _, ok, err := st.AcceptReport(ctx, "s", "audio", "d", "m", "a", "inc", "CM-ONE", now); err != nil || !ok {
		t.Fatalf("seed: %v %v", ok, err)
	}
	if _, ok, err := st.AcceptReport(ctx, "s", "audio", "d", "m", "a", "other", "CM-TWO", now.Add(25*time.Hour)); err != nil || !ok {
		t.Fatalf("25-hour duplicate still retained: accepted=%v err=%v", ok, err)
	}
}

func TestReviewP03DedupBoundAfterInsertion(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10000; i++ {
		if _, err := tx.Exec(`INSERT INTO report_dedup(key,retained_until) VALUES(?,?)`, fmt.Sprintf("old-%d", i), formatTime(now.Add(time.Hour))); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := st.AcceptReport(ctx, "s", "audio", "d", "m", "a", "inc", "CM-ONE", now); err != nil || !ok {
		t.Fatalf("accept: %v %v", ok, err)
	}
	var count int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM report_dedup`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count > 10000 {
		t.Fatalf("durable dedup has %d keys after commit, bound is 10000", count)
	}
}

func TestReviewP03FeedbackFailureIsAtomic(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	inc, err := st.CreateManualIncident(ctx, "s", "audio", "inc", "CM-ONE", now)
	if err != nil {
		t.Fatal(err)
	}
	inc.Stage = domain.StageAwaitingFeedback
	if err := st.SaveIncident(ctx, inc); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`CREATE TRIGGER review_fail_incident BEFORE UPDATE ON incidents BEGIN SELECT RAISE(ABORT,'review fixture failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddFeedback(ctx, inc.ID, "actor", "positive", now); err == nil {
		t.Fatal("fault not exercised")
	}
	var count int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM feedback`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed feedback transaction left %d committed vote rows", count)
	}
}
