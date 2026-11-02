package incidents

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
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
		Microphone:  &domain.InputRef{Name: "Mic", UUID: "mic", Context: "Collection"},
		StreamTrack: &track, TrackConfirmedAt: &confirmed,
	}}
	c := New(Options{
		Store: st,
		Read: func(context.Context, domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
			mute := true
			tracks := map[string]bool{"track1": false, "track2": false, "track3": false, "track4": false, "track5": false, "track6": false}
			scene := "Main"
			return &domain.OBSDiscovery{Generation: 1, ConfigRevision: 1, SceneCollection: "Collection", CollectionKnown: true, Inputs: []domain.OBSInput{{Name: "Mic", UUID: "mic"}}, Scenes: []domain.OBSScene{{Name: "Main", UUID: "scene"}}},
				&domain.OBSObservation{Generation: 1, ConfigRevision: 1, SceneCollection: "Collection", ObservedAt: now, Microphone: &domain.InputRef{Name: "Mic", UUID: "mic"}, Mute: &mute, Tracks: tracks, ProgramScene: scene}, nil
		},
		Settings: func() domain.Settings { return *settings },
		Status: func() Readiness {
			return Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, ResolutionReady: true}
		},
		Now: time.Now,
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
		Status: func() Readiness {
			return Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, ResolutionReady: true}
		},
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
	c, _, st := coordinatorFixture(t)
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
	inc.Stage = domain.StageAwaitingFeedback
	if err := st.SaveIncident(ctx, inc); err != nil {
		t.Fatal(err)
	}
	c.setIncident(inc)
	if reply := c.HandleFeedbackAt(ctx, "still_broken", inc.PublicID, "actor-1", 1, time.Now()); reply == "" {
		t.Fatal("negative feedback rejected")
	}
	if got, _ := c.Find(id); got.Feedback.Negative != 1 || got.Stage != domain.StageUnresolved {
		t.Fatalf("negative feedback = %+v", got)
	}
	if reply := c.HandleFeedbackAt(ctx, "fixed", inc.PublicID, "actor-1", 1, time.Now()); reply == "" {
		t.Fatal("positive replacement rejected")
	}
	if got, _ := c.Find(id); got.Feedback.Positive != 1 || got.Feedback.Negative != 0 {
		t.Fatalf("latest vote did not replace: %+v", got.Feedback)
	}
	if _, err := c.Dismiss(ctx, id); err != nil {
		t.Fatal(err)
	}
	_ = c.HandleFeedbackAt(ctx, "fixed", inc.PublicID, "actor-2", 1, time.Now())
	if got, _ := c.Find(id); got.Feedback.Positive != 1 {
		t.Fatal("terminal incident accepted new feedback")
	}
}

func TestCoordinatorLoadsRetainedRecordBeyondCompletedPage(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "retained.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().UTC()
	open, err := st.CreateManualIncident(ctx, "old-session", "audio", "open-incident", "CM-OPEN", now)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 510; i++ {
		inc, err := st.CreateManualIncident(ctx, "old-session", "video", fmt.Sprintf("done-%03d", i), fmt.Sprintf("CM-D%03d", i), now)
		if err != nil {
			t.Fatal(err)
		}
		inc.Stage = domain.StageDismissed
		if err := st.SaveIncident(ctx, inc); err != nil {
			t.Fatal(err)
		}
	}
	c, err := NewChecked(Options{Store: st, Read: func(context.Context, domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
		return nil, nil, context.Canceled
	}, Settings: func() domain.Settings { return domain.Settings{} }, Status: func() Readiness { return Readiness{} }})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if got, ok := c.Find(open.ID); !ok || got.ID != open.ID {
		t.Fatal("retained open record outside the completed-history page was not loaded")
	}
}

func TestCoordinatorStaleFeedbackAfterProviderReplacementIsDropped(t *testing.T) {
	c, _, st := coordinatorFixture(t)
	ctx := context.Background()
	session, err := c.Start(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	c.HandleIssue(ctx, "audio", "stale-fb-d", "stale-fb-m", "viewer", 1)
	c.wg.Wait()
	_, list := c.Snapshot()
	if len(list) == 0 {
		t.Fatal("no incident")
	}
	inc := list[0]
	inc.Stage = domain.StageAwaitingFeedback
	if err := st.SaveIncident(ctx, inc); err != nil {
		t.Fatal(err)
	}
	c.setIncident(inc)
	c.status = func() Readiness {
		return Readiness{TwitchConnected: true, TwitchGeneration: 2, OBSConnected: true, OBSGeneration: 1, ResolutionReady: true}
	}
	if reply := c.HandleFeedbackInSessionAt(ctx, "still_broken", inc.PublicID, "viewer", 1, time.Now(), session.ID); reply != "" {
		t.Fatal("stale-generation feedback was acknowledged")
	}
	got, _ := c.Find(inc.ID)
	if got.Feedback.Negative != 0 {
		t.Fatal("stale feedback changed durable vote")
	}
}

func TestCoordinatorAutomaticProviderReplacementInvalidatesJob(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	var generation atomic.Uint64
	generation.Store(1)
	c.status = func() Readiness {
		return Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, OBSGeneration: generation.Load()}
	}
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
		t.Fatal(err)
	}
	<-entered
	generation.Store(2)
	close(release)
	c.wg.Wait()
	got, _ := c.Find(inc.ID)
	if len(got.Findings) != 0 || got.Stage == domain.StageFindingAvailable || got.Stage == domain.StageAwaitingFeedback {
		t.Fatalf("replaced provider result became current: %+v", got)
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

func TestCoordinatorConcurrentStartReturnsOneSession(t *testing.T) {
	c, _, _ := coordinatorFixture(t)
	const workers = 8
	results := make(chan string, workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			session, err := c.Start(context.Background(), 1)
			results <- session.ID
			errs <- err
		}()
	}
	ids := map[string]bool{}
	for i := 0; i < workers; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		ids[<-results] = true
	}
	if len(ids) != 1 {
		t.Fatalf("concurrent starts created %d sessions", len(ids))
	}
}

func TestCoordinatorStopCancelsManualJobWithoutSession(t *testing.T) {
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "manual-stop.db"))
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
	c := New(Options{Store: st,
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
		Status: func() Readiness {
			return Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, ResolutionReady: true}
		},
	})
	defer c.Close()
	if _, err := c.Check(context.Background(), "audio", 1, ""); err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := c.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(release)
	c.wg.Wait()
	c.mu.Lock()
	jobs := len(c.jobs)
	c.mu.Unlock()
	if jobs != 0 {
		t.Fatal("manual job survived stop")
	}
}
