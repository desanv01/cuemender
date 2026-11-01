//go:build reviewrepro

package incidents

import (
	"context"
	"github.com/desanv01/cuemender/internal/domain"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReviewR01ClosedCoordinatorRejectsCheck(t *testing.T) {
	c, _, st := coordinatorFixture(t)
	c.Close()
	if _, err := c.Check(context.Background(), "audio", 1, ""); err == nil {
		t.Error("closed coordinator accepted and persisted a new check")
	}
	list, err := st.ListIncidents(context.Background(), 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("closed coordinator left %d orphaned incidents", len(list))
	}
}

func TestReviewR01ManualResultRejectsReplacedOBS(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	var generation atomic.Uint64
	generation.Store(1)
	c.status = func() Readiness { return Readiness{OBSConnected: true, OBSGeneration: generation.Load()} }
	entered, release := make(chan struct{}), make(chan struct{})
	original := c.read
	c.read = func(ctx context.Context, s domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
		d, o, err := original(ctx, s)
		close(entered)
		<-release
		return d, o, err
	}
	inc, err := c.Check(context.Background(), "audio", 1, "")
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	<-entered
	generation.Store(2)
	close(release)
	c.wg.Wait()
	got, _ := c.Find(inc.ID)
	if len(got.Findings) != 0 {
		t.Fatalf("manual result from replaced generation installed %d findings", len(got.Findings))
	}
}

func TestReviewR01ManualStopRecordsInterrupted(t *testing.T) {
	c, _, st := coordinatorFixture(t)
	entered := make(chan struct{})
	c.read = func(ctx context.Context, _ domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
		close(entered)
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	inc, err := c.Check(context.Background(), "audio", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := c.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	c.wg.Wait()
	got, err := st.GetIncident(context.Background(), inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stage == domain.StageInvestigating {
		t.Fatal("manual stop left durable investigating incident with no job")
	}
}

func TestReviewR01CanceledReadRetainsCategoryAdmission(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	entered, release := make(chan struct{}, 2), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	var active, maxActive atomic.Int32
	c.read = func(ctx context.Context, _ domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := maxActive.Load(); n > old; old = maxActive.Load() {
			if maxActive.CompareAndSwap(old, n) {
				break
			}
		}
		entered <- struct{}{}
		<-release // models transport cleanup that has not joined after cancellation
		return nil, nil, ctx.Err()
	}
	if _, err := c.Check(context.Background(), "audio", 1, ""); err != nil {
		t.Fatal(err)
	}
	<-entered
	stopped := make(chan error, 1)
	go func() { stopped <- c.Stop(context.Background()) }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Check(context.Background(), "audio", 1, ""); err == nil {
			select {
			case <-entered:
			case <-time.After(100 * time.Millisecond):
			}
		}
	case <-time.After(100 * time.Millisecond):
		// Joining cleanup in Stop is also valid. Let the transport finish.
		unblock()
		if err := <-stopped; err != nil {
			t.Fatal(err)
		}
	}
	if maxActive.Load() > 1 {
		t.Fatal("a second audio read started before the canceled first read joined")
	}
}
