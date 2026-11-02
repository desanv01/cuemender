package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

func TestP06ClearHistoryPreservesActiveAndPendingRecovery(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	mk := func(name string) domain.Incident {
		inc, err := st.CreateManualIncident(ctx, "s", "audio", name, "CM-"+name, now)
		if err != nil {
			t.Fatal(err)
		}
		return inc
	}
	done := mk("DONE")
	done.Stage = domain.StageDismissed
	if err := st.SaveIncident(ctx, done); err != nil {
		t.Fatal(err)
	}
	closed := mk("CLOSED")
	closed.Stage = domain.StageClosed
	closed.ClosureOutcome = "unresolved"
	if err := st.SaveIncident(ctx, closed); err != nil {
		t.Fatal(err)
	}
	active := mk("ACTIVE")
	active.Stage = domain.StageInvestigating
	if err := st.SaveIncident(ctx, active); err != nil {
		t.Fatal(err)
	}
	p := domain.Proposal{ID: "p06-pending", IncidentID: closed.ID, Status: domain.ProposalReserved}
	a := domain.Action{ID: "p06-action", ProposalID: p.ID, IncidentID: closed.ID, Status: domain.ActionReserved}
	if err := st.SaveRepair(ctx, &p, &a, &closed, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClearHistory(ctx); !errors.Is(err, domain.ErrClosureIneligible) {
		t.Fatalf("clear with pending action = %v", err)
	}
	var count int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM incidents`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("pending clear changed history: count=%d err=%v", count, err)
	}
	a.Status = domain.TrackRepairVerified
	if err := st.SaveRepair(ctx, nil, &a, nil, false); err != nil {
		t.Fatal(err)
	}
	n, err := st.ClearHistory(ctx)
	if err != nil || n != 2 {
		t.Fatalf("clear=%d err=%v", n, err)
	}
	if _, err := st.GetIncident(ctx, active.ID); err != nil {
		t.Fatal("active incident was cleared")
	}
	for _, id := range []string{done.ID, closed.ID} {
		if _, err := st.GetIncident(ctx, id); err == nil {
			t.Fatalf("completed incident %s survived clear", id)
		}
	}
}

func TestP06HistoryFailureRollsBackDeletion(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	inc, err := st.CreateManualIncident(ctx, "s", "audio", "rollback", "CM-ROLLBK", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	inc.Stage = domain.StageDismissed
	if err := st.SaveIncident(ctx, inc); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`CREATE TRIGGER reject_history BEFORE DELETE ON incidents BEGIN SELECT RAISE(ABORT, 'test disk failure'); END`); err != nil {
		t.Fatal(err)
	}
	if ids, err := st.ClearHistoryIDs(ctx); err == nil || len(ids) != 0 {
		t.Fatalf("failed delete ids=%v err=%v", ids, err)
	}
	if _, err := st.GetIncident(ctx, inc.ID); err != nil {
		t.Fatal("failed clear removed durable incident", err)
	}
	if _, err := st.db.Exec(`DROP TRIGGER reject_history`); err != nil {
		t.Fatal(err)
	}
	ids, err := st.ClearHistoryIDs(ctx)
	if err != nil || len(ids) != 1 || ids[0] != inc.ID {
		t.Fatalf("retry ids=%v err=%v", ids, err)
	}
}
