//go:build reviewrepro

package incidents

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/store"
)

func TestReviewR03CanceledProductionGateReturns(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	if err := c.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { _, err := c.Check(ctx, "audio", 1, ""); done <- err }()
	select {
	case err := <-done:
		c.release()
		if err == nil {
			t.Fatal("canceled operation succeeded")
		}
	case <-time.After(150 * time.Millisecond):
		c.release()
		<-done
		t.Fatal("canceled operation did not return while production gate was occupied")
	}
}

func TestReviewR03RetainedHistorySurvivesDismissReload(t *testing.T) {
	c, _, st := coordinatorFixture(t)
	ctx := context.Background()
	for i := 0; i < 502; i++ {
		id := fmt.Sprintf("retained-%03d", i)
		if _, err := st.CreateManualIncident(ctx, "old", "audio", id, "CM-"+id, time.Now().Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	c.Close()
	next, err := NewChecked(Options{Store: st, Read: c.read, Settings: c.settings, Status: c.status})
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	_, before := next.Snapshot()
	if len(before) != 502 {
		t.Fatalf("precondition: loaded %d retained rows", len(before))
	}
	if _, err := next.Dismiss(ctx, before[0].ID); err != nil {
		t.Fatal(err)
	}
	_, after := next.Snapshot()
	if len(after) != 502 {
		t.Fatalf("dismiss/reload hid retained records: before=%d after=%d, want all 501 open plus one completed", len(before), len(after))
	}
}

func TestReviewR03ManualStopRetriesAfterRecoverableSQLFault(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "retry.db")
	st, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	entered := make(chan struct{})
	c, err := NewChecked(Options{Store: st,
		Settings: func() domain.Settings { return domain.Settings{Revision: 1} },
		Status:   func() Readiness { return Readiness{OBSConnected: true} },
		Read: func(ctx context.Context, _ domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
			close(entered)
			<-ctx.Done()
			return nil, nil, ctx.Err()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	inc, err := c.Check(ctx, "audio", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	if _, err := db.Exec(`CREATE TRIGGER review_stop_failure BEFORE UPDATE ON incidents BEGIN SELECT RAISE(ABORT,'review stop failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := c.Stop(ctx); err == nil {
		t.Fatal("stop did not report injected durable failure")
	}
	c.wg.Wait()
	if _, err := db.Exec(`DROP TRIGGER review_stop_failure`); err != nil {
		t.Fatal(err)
	}
	if err := c.Stop(ctx); err != nil {
		t.Fatalf("retry failed after fault removed: %v", err)
	}
	durable, err := st.GetIncident(ctx, inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if durable.Stage != domain.StageInterrupted {
		t.Fatalf("stop retry reported success but durable stage=%s, want interrupted", durable.Stage)
	}
}

func TestReviewR03SuccessfulJobPublishesSettledCount(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	var last atomic.Int64
	last.Store(-1)
	c.onChange = func() { last.Store(int64(c.ActiveJobs())) }
	inc, err := c.Check(context.Background(), "audio", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	c.wg.Wait()
	got, ok := c.Find(inc.ID)
	if !ok || len(got.Findings) == 0 {
		t.Fatalf("precondition: successful finding missing: %+v", got)
	}
	if c.ActiveJobs() != 0 {
		t.Fatal("job did not finish")
	}
	if last.Load() != 0 {
		t.Fatalf("last published job count=%d after completion; want zero without unrelated poll", last.Load())
	}
}

func TestReviewR03ClosePreventsLateStartCommit(t *testing.T) {
	c, settings, st := coordinatorFixture(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	c.settings = func() domain.Settings { close(entered); <-release; return *settings }
	result := make(chan error, 1)
	go func() { _, err := c.Start(context.Background(), 1); result <- err }()
	<-entered
	closed := make(chan struct{})
	go func() { c.Close(); close(closed) }()
	returnedBeforeStart := false
	select {
	case <-closed:
		returnedBeforeStart = true
		close(release)
	case <-time.After(100 * time.Millisecond):
		// A correct close may join the admitted operation rather than return first.
		close(release)
		select {
		case <-closed:
		case <-time.After(2 * time.Second):
			t.Fatal("close did not join admitted start")
		}
	}
	err := <-result
	durable, readErr := st.ActiveMonitoringSession(context.Background())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if returnedBeforeStart && (err == nil || durable != nil) {
		t.Fatalf("start survived close: err=%v durable active=%v", err, durable != nil)
	}
}
