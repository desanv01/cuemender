//go:build reviewrepro

package incidents

import (
	"context"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

func TestReviewR02CanceledGateReturnsPromptly(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	c.opMu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { _, err := c.Check(ctx, "audio", 1, ""); done <- err }()
	select {
	case err := <-done:
		c.opMu.Unlock()
		if err == nil {
			t.Fatal("canceled check succeeded")
		}
	case <-time.After(150 * time.Millisecond):
		c.opMu.Unlock()
		<-done
		t.Fatal("canceled command waited for the operation gate instead of returning")
	}
}

func TestReviewR02DisconnectedTwitchRejectsQueuedReport(t *testing.T) {
	c, _, st := coordinatorFixture(t)
	if _, err := c.Start(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	c.status = func() Readiness {
		return Readiness{TwitchConnected: false, TwitchGeneration: 0, OBSConnected: true, OBSGeneration: 1, ResolutionReady: true}
	}
	c.HandleIssueAt(context.Background(), "audio", "stale-d", "stale-m", "actor", 1, time.Now())
	c.wg.Wait()
	list, err := st.ListIncidents(context.Background(), 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) > 0 {
		t.Fatal("report from a disconnected Twitch generation was durably accepted")
	}
}

func TestReviewR02ManualStopFailureIsReported(t *testing.T) {
	c, _, st := coordinatorFixture(t)
	entered := make(chan struct{})
	c.read = func(ctx context.Context, _ domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
		close(entered)
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	if _, err := c.Check(context.Background(), "audio", 1, ""); err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Stop(context.Background()); err == nil {
		t.Error("manual stop reported success after durable interruption failed")
	}
	c.Close()
}
