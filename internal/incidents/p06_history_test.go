package incidents

import (
	"context"
	"errors"
	"testing"

	"github.com/desanv01/cuemender/internal/domain"
)

func TestP06ClearHistoryPublishesCommittedCachesAndPreservesOpen(t *testing.T) {
	f := newRepairFixture(t)
	ctx := context.Background()
	seed := func(id, stage string) domain.Incident {
		inc, err := f.st.CreateManualIncident(ctx, "history", "audio", id, "CM-"+id, f.now())
		if err != nil {
			t.Fatal(err)
		}
		inc.Stage = stage
		if err := f.st.SaveIncident(ctx, inc); err != nil {
			t.Fatal(err)
		}
		f.c.setIncident(inc)
		return inc
	}
	done := seed("DONE", domain.StageClosed)
	open := seed("OPEN", domain.StageUnresolved)
	p := domain.Proposal{ID: "done-proposal", IncidentID: done.ID, Status: domain.ProposalInvalidated}
	if err := f.st.SaveRepair(ctx, &p, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	f.c.proposals[p.ID] = p
	publications := 0
	f.c.onChange = func() {
		publications++
		if _, exists := f.c.Find(done.ID); exists {
			t.Error("published deleted incident")
		}
		if _, exists := f.c.Find(open.ID); !exists {
			t.Error("lost unresolved incident")
		}
		f.c.mu.Lock()
		defer f.c.mu.Unlock()
		if _, exists := f.c.proposals[p.ID]; exists {
			t.Error("published deleted proposal")
		}
		if len(f.c.order) != 1 || f.c.order[0] != open.ID {
			t.Error("stale ordered history")
		}
	}
	n, err := f.c.ClearHistory(ctx)
	if err != nil || n != 1 || publications != 1 {
		t.Fatalf("clear=%d err=%v publishes=%d", n, err, publications)
	}
	if _, err := f.st.GetIncident(ctx, done.ID); err == nil {
		t.Fatal("deleted cache but not SQL")
	}
	if _, err := f.st.GetIncident(ctx, open.ID); err != nil {
		t.Fatal(err)
	}
	ps, _, err := f.st.LoadRepairs(ctx)
	if err != nil || len(ps) != 0 {
		t.Fatalf("orphan proposal: %v %v", ps, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := f.c.ClearHistory(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
	if publications != 1 {
		t.Fatal("canceled clear published")
	}
	// Admission must not clear history while canceled work still owns settlement.
	f.c.pending[open.ID] = open
	if _, err := f.c.ClearHistory(ctx); !errors.Is(err, domain.ErrClosureIneligible) {
		t.Fatalf("pending=%v", err)
	}
	delete(f.c.pending, open.ID)
}
