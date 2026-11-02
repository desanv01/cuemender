package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

func TestModeOwnershipRejectsCrossMode(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "mode.db")
	rehearsal, err := OpenMode(ctx, path, domain.ModeRehearsal)
	if err != nil {
		t.Fatal(err)
	}
	if err := rehearsal.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenMode(ctx, path, domain.ModeLive); !errors.Is(err, domain.ErrModeConflict) {
		t.Fatalf("live open of rehearsal DB = %v", err)
	}
	livePath := filepath.Join(t.TempDir(), "live.db")
	live, err := OpenMode(ctx, livePath, domain.ModeLive)
	if err != nil {
		t.Fatal(err)
	}
	if err := live.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenMode(ctx, livePath, domain.ModeRehearsal); !errors.Is(err, domain.ErrModeConflict) {
		t.Fatalf("rehearsal open of marked live DB = %v", err)
	}
}

func TestCloseIncidentTransactionalPredicates(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	inc, err := st.CreateManualIncident(ctx, "sess-close", "audio", "close-inc", "CM-CLOSE", now)
	if err != nil {
		t.Fatal(err)
	}
	inc.Stage = domain.StageAwaitingFeedback
	inc.LocalVerification = domain.LocalVerified
	if err := st.SaveIncident(ctx, inc); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddFeedback(ctx, inc.ID, "viewer-1", "positive", now); err != nil {
		t.Fatal(err)
	}
	closed, err := st.CloseIncident(ctx, inc.ID, "viewer_confirmed", "sess-close", now.Add(time.Second))
	if err != nil || closed.Stage != domain.StageClosed || closed.ClosureOutcome != "viewer_confirmed" {
		t.Fatalf("close=%+v err=%v", closed, err)
	}
	if again, err := st.CloseIncident(ctx, inc.ID, "viewer_confirmed", "sess-close", now.Add(2*time.Second)); err != nil || again.ClosureOutcome != "viewer_confirmed" {
		t.Fatalf("idempotent close=%+v err=%v", again, err)
	}
	if _, err := st.CloseIncident(ctx, inc.ID, "unresolved", "sess-close", now.Add(3*time.Second)); !errors.Is(err, domain.ErrClosureConflict) {
		t.Fatalf("conflicting close = %v", err)
	}
	other, err := st.CreateManualIncident(ctx, "sess-close", "video", "close-inc-2", "CM-CLOSE2", now)
	if err != nil {
		t.Fatal(err)
	}
	other.Stage = domain.StageFindingAvailable
	if err := st.SaveIncident(ctx, other); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CloseIncident(ctx, other.ID, "local_only", "sess-close", now); !errors.Is(err, domain.ErrClosureFeedback) {
		t.Fatalf("unverified local_only = %v", err)
	}
}
