package incidents

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/store"
)

func coordinatorFixture(t *testing.T) (*Coordinator, *domain.Settings, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "incidents.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	now := time.Now().UTC()
	track := 2
	confirmed := now
	settings := &domain.Settings{Revision: 1, Expectations: domain.Expectations{
		Microphone:  &domain.InputRef{Name: "Mic", UUID: "mic"},
		StreamTrack: &track, TrackConfirmedAt: &confirmed,
	}}
	c := New(Options{
		Store: st,
		Read: func(context.Context, domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
			mute := true
			tracks := map[string]bool{"track1": false, "track2": false, "track3": false, "track4": false, "track5": false, "track6": false}
			scene := "Main"
			return &domain.OBSDiscovery{Generation: 1, ConfigRevision: 1, Inputs: []domain.OBSInput{{Name: "Mic", UUID: "mic"}}, Scenes: []domain.OBSScene{{Name: "Main", UUID: "scene"}}},
				&domain.OBSObservation{Generation: 1, ConfigRevision: 1, ObservedAt: now, Mute: &mute, Tracks: tracks, ProgramScene: scene}, nil
		},
		Settings: func() domain.Settings { return *settings },
		Status:   func() Readiness { return Readiness{TwitchConnected: true, OBSConnected: true, ResolutionReady: true} },
		Now:      time.Now,
	})
	t.Cleanup(c.Close)
	return c, settings, st
}

func waitIncident(t *testing.T, c *Coordinator, id string, wantStage string) domain.Incident {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if inc, ok := c.Find(id); ok && inc.Stage == wantStage {
			return inc
		}
		time.Sleep(5 * time.Millisecond)
	}
	inc, _ := c.Find(id)
	t.Fatalf("incident %s stage = %s, want %s (%+v)", id, inc.Stage, wantStage, inc)
	return domain.Incident{}
}

func TestCoordinatorStartIssueFindingStop(t *testing.T) {
	c, _, st := coordinatorFixture(t)
	ctx := context.Background()
	session, err := c.Start(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !session.Active {
		t.Fatal("session did not start")
	}
	again, _ := c.Start(ctx, 1)
	if again.ID != session.ID {
		t.Fatal("repeat start created a second session")
	}
	reply := c.HandleIssue(ctx, "audio", "d1", "m1", "viewer-1", 1)
	if reply == "" {
		t.Fatal("accepted report produced no acknowledgment")
	}
	inc := waitIncident(t, c, firstIncidentID(t, c), domain.StageFindingAvailable)
	if len(inc.Findings) != 2 {
		t.Fatalf("findings = %+v, want muted + track missing", inc.Findings)
	}
	if c.HandleIssue(ctx, "audio", "d1", "m2", "viewer-1", 1) != "" {
		t.Fatal("duplicate delivery produced a second acknowledgment")
	}
	if err := c.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	sess, _ := c.Snapshot()
	if sess == nil || sess.Active {
		t.Fatal("stop left monitoring active")
	}
	active, _ := st.ActiveMonitoringSession(ctx)
	if active != nil {
		t.Fatal("durable session remained active")
	}
}

func firstIncidentID(t *testing.T, c *Coordinator) string {
	t.Helper()
	_, list := c.Snapshot()
	if len(list) == 0 {
		t.Fatal("no incident")
	}
	return list[len(list)-1].ID
}

func TestCoordinatorManualCheckWhileStoppedAndDismiss(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	ctx := context.Background()
	inc, err := c.Check(ctx, "audio", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if inc.SessionID == "" || inc.ReportCount != 0 || inc.DistinctReporters != 0 {
		t.Fatalf("manual incident copied viewer counts: %+v", inc)
	}
	finished := waitIncident(t, c, inc.ID, domain.StageFindingAvailable)
	if len(finished.Findings) == 0 {
		t.Fatal("manual check produced no finding")
	}
	if _, err := c.Dismiss(ctx, inc.ID); err != nil {
		t.Fatal(err)
	}
	if inc, _ := c.Find(inc.ID); inc.Stage != domain.StageDismissed {
		t.Fatalf("stage = %s", inc.Stage)
	}
}

func TestCoordinatorStopRejectsLateBlockedRead(t *testing.T) {
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "blocked.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().UTC()
	track := 2
	confirmed := now
	settings := domain.Settings{Revision: 1, Expectations: domain.Expectations{
		Microphone: &domain.InputRef{Name: "Mic", UUID: "mic"}, StreamTrack: &track, TrackConfirmedAt: &confirmed,
	}}
	entered := make(chan struct{})
	release := make(chan struct{})
	c := New(Options{
		Store: st,
		Read: func(ctx context.Context, _ domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
			close(entered)
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-release:
			}
			return nil, nil, context.Canceled
		},
		Settings: func() domain.Settings { return settings },
		Status:   func() Readiness { return Readiness{TwitchConnected: true, OBSConnected: true, ResolutionReady: true} },
	})
	defer c.Close()
	if _, err := c.Start(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	reply := c.HandleIssue(context.Background(), "audio", "d1", "m1", "viewer", 1)
	if reply == "" {
		t.Fatal("report not durably accepted")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("diagnostic read did not start")
	}
	if err := c.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(release)
	time.Sleep(50 * time.Millisecond)
	_, list := c.Snapshot()
	for _, inc := range list {
		if inc.Stage == domain.StageFindingAvailable || len(inc.Findings) > 0 {
			t.Fatalf("late blocked read mutated stopped incident: %+v", inc)
		}
	}
}

func TestCoordinatorFeedbackLatestVoteAndTerminalRejection(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	ctx := context.Background()
	if _, err := c.Start(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if reply := c.HandleIssue(ctx, "audio", "d1", "m1", "viewer", 1); reply == "" {
		t.Fatal("issue not accepted")
	}
	_, list := c.Snapshot()
	id := list[len(list)-1].ID
	inc := waitIncident(t, c, id, domain.StageFindingAvailable)
	if reply := c.HandleFeedback(ctx, "still_broken", inc.PublicID, "actor-1"); reply == "" {
		t.Fatal("negative feedback rejected")
	}
	if got, _ := c.Find(id); got.Feedback.Negative != 1 || got.Stage != domain.StageUnresolved {
		t.Fatalf("negative feedback = %+v", got)
	}
	if reply := c.HandleFeedback(ctx, "fixed", inc.PublicID, "actor-1"); reply == "" {
		t.Fatal("positive replacement rejected")
	}
	if got, _ := c.Find(id); got.Feedback.Positive != 1 || got.Feedback.Negative != 0 {
		t.Fatalf("latest vote did not replace: %+v", got.Feedback)
	}
	if _, err := c.Dismiss(ctx, id); err != nil {
		t.Fatal(err)
	}
	_ = c.HandleFeedback(ctx, "fixed", inc.PublicID, "actor-2")
	if got, _ := c.Find(id); got.Feedback.Positive != 1 {
		t.Fatal("terminal incident accepted new feedback")
	}
}

func TestCoordinatorStartRequiresTrackConfirmation(t *testing.T) {
	c, settings, _ := coordinatorFixture(t)
	settings.TrackConfirmedAt = nil
	_, err := c.Start(context.Background(), 1)
	if err == nil {
		t.Fatal("monitoring started without explicit track confirmation")
	}
}
