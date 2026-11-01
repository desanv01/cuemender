//go:build reviewrepro

package incidents

import (
	"context"
	"errors"
	"github.com/desanv01/cuemender/internal/domain"
	"sync/atomic"
	"testing"
)

func TestReviewP03CategoryIsolation(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	inc, err := c.Check(context.Background(), "video", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	c.wg.Wait()
	got, _ := c.Find(inc.ID)
	for _, f := range got.Findings {
		if f.Category != "video" {
			t.Fatalf("video incident contains %s finding", f.Code)
		}
	}
	if got.Stage != domain.StageInsufficientInfo {
		t.Fatalf("unconfigured video should be insufficient, got %s", got.Stage)
	}
}

func TestReviewP03ActiveManualJoinsSession(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	session, err := c.Start(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	inc, err := c.Check(context.Background(), "audio", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	c.wg.Wait()
	if inc.SessionID != session.ID {
		t.Fatalf("active manual check created detached session %s instead of %s", inc.SessionID, session.ID)
	}
}

func TestReviewP03ActiveStartChecksRevision(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	if _, err := c.Start(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Start(context.Background(), 0); !errors.Is(err, domain.ErrRevisionConflict) {
		t.Fatalf("stale repeat start = %v, want revision conflict", err)
	}
}

func TestReviewP03RejectIneligibleFeedback(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	if _, err := c.Start(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	c.HandleIssue(context.Background(), "audio", "d", "m", "a", 1)
	c.wg.Wait()
	_, list := c.Snapshot()
	inc := list[0]
	if inc.Stage != domain.StageFindingAvailable {
		t.Fatalf("fixture stage %s", inc.Stage)
	}
	c.HandleFeedback(context.Background(), "fixed", inc.PublicID, "b")
	got, _ := c.Find(inc.ID)
	if got.Feedback.Positive != 0 {
		t.Fatal("feedback accepted before awaiting_feedback/unresolved eligibility")
	}
}

func TestReviewP03DismissDoesNotCancelOtherIncident(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	old, err := c.Check(context.Background(), "audio", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	c.wg.Wait()
	entered := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	var canceled atomic.Bool
	c.read = func(ctx context.Context, _ domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
		close(entered)
		<-release
		canceled.Store(ctx.Err() != nil)
		return nil, nil, ctx.Err()
	}
	current, err := c.Check(context.Background(), "audio", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	if _, err := c.Dismiss(context.Background(), old.ID); err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	job := c.jobs["audio"]
	c.mu.Unlock()
	if job == nil || job.incident.ID != current.ID || job.ctx.Err() != nil {
		t.Fatal("dismissing an older incident canceled the different current audio job")
	}
}

func TestReviewP03ReportsCoalesceCompletedDiagnosis(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	original := c.read
	var reads atomic.Int32
	c.read = func(ctx context.Context, settings domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
		reads.Add(1)
		return original(ctx, settings)
	}
	if _, err := c.Start(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	c.HandleIssue(context.Background(), "audio", "d1", "m1", "a", 1)
	c.wg.Wait()
	c.HandleIssue(context.Background(), "audio", "d2", "m2", "b", 1)
	c.wg.Wait()
	if reads.Load() != 1 {
		t.Fatalf("joining report triggered %d reads; expected existing finding until explicit recheck", reads.Load())
	}
}

func TestReviewP03ProviderStatusOutsideCoordinatorLock(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	var calledUnderLock atomic.Bool
	c.status = func() Readiness {
		if c.mu.TryLock() {
			c.mu.Unlock()
		} else {
			calledUnderLock.Store(true)
		}
		return Readiness{TwitchConnected: true, OBSConnected: true, ResolutionReady: true}
	}
	if _, err := c.Check(context.Background(), "audio", 1, ""); err != nil {
		t.Fatal(err)
	}
	c.wg.Wait()
	if calledUnderLock.Load() {
		t.Fatal("status callback called under coordinator mutex; production status takes app.stateMu while snapshots take the reverse lock order")
	}
}

func TestReviewP03FailedStopDoesNotSilentlyDiverge(t *testing.T) {
	c, _, st := coordinatorFixture(t)
	if _, err := c.Start(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Stop(context.Background()); err == nil {
		t.Fatal("closed database did not exercise failure")
	}
	if err := c.Stop(context.Background()); err == nil {
		t.Fatal("retry silently succeeded although durable stop never committed")
	}
}
