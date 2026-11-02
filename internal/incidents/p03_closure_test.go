package incidents

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

func TestClosureSettlementSerializesDismissAndRecheck(t *testing.T) {
	for _, operation := range []string{"dismiss", "recheck"} {
		t.Run(operation, func(t *testing.T) {
			c, _, st := coordinatorFixture(t)
			fixed := time.Now().UTC()
			c.now = func() time.Time { return fixed }
			if _, err := c.Start(context.Background(), 1); err != nil {
				t.Fatal(err)
			}
			entered := make(chan struct{}, 2)
			c.read = func(ctx context.Context, _ domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
				entered <- struct{}{}
				<-ctx.Done()
				return nil, nil, ctx.Err()
			}
			inc, err := c.Check(context.Background(), "audio", 1, "")
			if err != nil {
				t.Fatal(err)
			}
			<-entered
			publishing := make(chan struct{})
			release := make(chan struct{})
			var unblock sync.Once
			t.Cleanup(func() { unblock.Do(func() { close(release) }) })
			var blocked atomic.Bool
			c.onChange = func() {
				state, _ := c.Find(inc.ID)
				if state.Stage == domain.StageInterrupted && c.ActiveJobs() == 0 && blocked.CompareAndSwap(false, true) {
					close(publishing)
					<-release
				}
			}
			c.mu.Lock()
			c.jobs["audio"].cancel()
			c.mu.Unlock()
			select {
			case <-publishing:
			case <-time.After(3 * time.Second):
				t.Fatal("settlement did not reach publication barrier")
			}
			started := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				close(started)
				if operation == "dismiss" {
					_, err := c.Dismiss(context.Background(), inc.ID)
					done <- err
				} else {
					_, err := c.Check(context.Background(), "audio", 1, inc.ID)
					done <- err
				}
			}()
			<-started
			select {
			case err := <-done:
				t.Fatalf("%s escaped the settlement publication gate: %v", operation, err)
			case <-time.After(80 * time.Millisecond):
			}
			unblock.Do(func() { close(release) })
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("operation did not join after settlement")
			}
			if operation == "recheck" {
				select {
				case <-entered:
				case <-time.After(time.Second):
					t.Fatal("new recheck did not start")
				}
			}
			want := domain.StageDismissed
			if operation == "recheck" {
				want = domain.StageInvestigating
			}
			cached, _ := c.Find(inc.ID)
			durable, err := st.GetIncident(context.Background(), inc.ID)
			if err != nil {
				t.Fatal(err)
			}
			if cached.Stage != want || durable.Stage != want {
				t.Fatalf("cache=%s durable=%s want %s", cached.Stage, durable.Stage, want)
			}
		})
	}
}

func TestClosurePendingRetryDoesNotAdoptNewerVersion(t *testing.T) {
	c, _, st := coordinatorFixture(t)
	ctx := context.Background()
	old, err := st.CreateManualIncident(ctx, "manual-old", "audio", "pending-old", "CM-PEND", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	old.Stage = domain.StageInvestigating
	if err := st.SaveIncident(ctx, old); err != nil {
		t.Fatal(err)
	}
	c.pending[old.ID] = old
	newer := old
	newer.UpdatedAt = old.UpdatedAt.Add(time.Second)
	if err := st.SaveIncident(ctx, newer); err != nil {
		t.Fatal(err)
	}
	c.setIncident(newer)
	if err := c.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetIncident(ctx, old.ID)
	if err != nil {
		t.Fatal(err)
	}
	cached, _ := c.Find(old.ID)
	if got.Stage != domain.StageInvestigating || cached.Stage != got.Stage || !got.UpdatedAt.Equal(newer.UpdatedAt) {
		t.Fatalf("old pending retry adopted newer work: cache=%s durable=%+v", cached.Stage, got)
	}
}

func TestClosureConcurrentCloseWaitsForSettlement(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	entered := make(chan struct{})
	canceled := make(chan struct{})
	release := make(chan struct{})
	var unblock sync.Once
	t.Cleanup(func() { unblock.Do(func() { close(release) }) })
	c.read = func(ctx context.Context, _ domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
		close(entered)
		<-ctx.Done()
		close(canceled)
		<-release
		return nil, nil, ctx.Err()
	}
	if _, err := c.Check(context.Background(), "audio", 1, ""); err != nil {
		t.Fatal(err)
	}
	<-entered
	first, second := make(chan struct{}), make(chan struct{})
	go func() { c.Close(); close(first) }()
	<-canceled
	go func() { c.Close(); close(second) }()
	select {
	case <-second:
		t.Fatal("second Close returned before worker joined")
	case <-time.After(80 * time.Millisecond):
	}
	unblock.Do(func() { close(release) })
	for _, done := range []chan struct{}{first, second} {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("Close deadlocked with settlement")
		}
	}
	if c.ActiveJobs() != 0 {
		t.Fatal("jobs remain after Close")
	}
}
