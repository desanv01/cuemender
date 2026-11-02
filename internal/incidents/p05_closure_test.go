package incidents

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

func p05WaitIdle(t *testing.T, c *Coordinator) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		idle := len(c.jobs) == 0 && !c.actionRunning
		c.mu.Unlock()
		if idle {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("coordinator did not release owned work")
}

func p05Replacement(f *repairFixture, installs *atomic.Int32) (domain.Settings, error) {
	f.mu.Lock()
	settings := f.settings
	f.mu.Unlock()
	exp := settings.Expectations
	track := 3
	exp.StreamTrack = &track
	return f.c.ReplaceRehearsal(context.Background(), settings.Revision, exp, func(updated domain.Settings, caches func()) {
		f.mu.Lock()
		f.settings = updated
		f.mu.Unlock()
		caches()
		installs.Add(1)
	})
}

func p05WaitSignal(t *testing.T, ch <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func TestP05ScenarioRejectsBlockedDiagnosis(t *testing.T) {
	f := newRepairFixtureMode(t, domain.ModeRehearsal)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	f.c.read = func(ctx context.Context, _ domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
		close(entered)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-release:
			return nil, nil, domain.ErrUnavailable
		}
	}
	session, err := f.c.Start(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	inc, err := f.c.Check(context.Background(), "audio", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	p05WaitSignal(t, entered, "blocked diagnosis")
	var installs atomic.Int32
	if _, err := p05Replacement(f, &installs); !errors.Is(err, domain.ErrBusy) {
		t.Fatalf("scenario during diagnosis = %v", err)
	}
	if got := f.c.ActiveSession(); got == nil || got.ID != session.ID || !got.Active || installs.Load() != 0 {
		t.Fatalf("rejected scenario changed session/install: %+v installs=%d", got, installs.Load())
	}
	if got, _ := f.c.Find(inc.ID); got.Stage != domain.StageInvestigating {
		t.Fatalf("rejected scenario interrupted diagnosis: %+v", got)
	}
	once.Do(func() { close(release) })
	p05WaitIdle(t, f.c)
}

func TestP05ScenarioRejectsBlockedApproval(t *testing.T) {
	for _, phase := range []string{"before_dispatch", "after_dispatch"} {
		t.Run(phase, func(t *testing.T) {
			f := newRepairFixtureMode(t, domain.ModeRehearsal)
			p := f.proposal(t)
			p05WaitIdle(t, f.c)
			a, err := f.c.Approve(context.Background(), p.ID, p.ConfigRevision)
			if err != nil {
				t.Fatal(err)
			}
			p05WaitSignal(t, f.before, "repair preflight")
			if phase == "after_dispatch" {
				close(f.resume)
				p05WaitSignal(t, f.sent, "admitted dispatch")
			}
			beforeSession := f.c.ActiveSession()
			beforeSettings, err := f.st.LoadSettings(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var installs atomic.Int32
			if _, err := p05Replacement(f, &installs); !errors.Is(err, domain.ErrBusy) {
				t.Fatalf("scenario during %s = %v", phase, err)
			}
			afterSettings, err := f.st.LoadSettings(context.Background())
			if err != nil || !reflect.DeepEqual(beforeSettings, afterSettings) || !reflect.DeepEqual(beforeSession, f.c.ActiveSession()) || installs.Load() != 0 {
				t.Fatalf("busy scenario changed durable settings/session/install: %v installs=%d", err, installs.Load())
			}
			wantWrites := int32(0)
			if phase == "after_dispatch" {
				wantWrites = 1
			}
			if f.writes.Load() != wantWrites {
				t.Fatalf("writes=%d want %d", f.writes.Load(), wantWrites)
			}
			if phase == "before_dispatch" {
				close(f.resume)
			}
			close(f.finish)
			waitAction(t, f.c, a.ID, domain.TrackRepairVerified)
			p05WaitIdle(t, f.c)
		})
	}
}

func TestP05ScenarioRejectsReconciliationAndCloseJoins(t *testing.T) {
	f := newRepairFixtureMode(t, domain.ModeRehearsal)
	f.unknown = true
	p := f.proposal(t)
	p05WaitIdle(t, f.c)
	a, err := f.c.Approve(context.Background(), p.ID, p.ConfigRevision)
	if err != nil {
		t.Fatal(err)
	}
	close(f.resume)
	close(f.finish)
	waitAction(t, f.c, a.ID, domain.TrackRepairUnknown)
	p05WaitIdle(t, f.c)
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	f.c.readTrack = func(ctx context.Context, _ domain.TrackStateRequest) (domain.TrackState, error) {
		close(entered)
		<-ctx.Done()
		close(canceled)
		<-release
		return domain.TrackState{}, ctx.Err()
	}
	// Own the reconciliation in the same wait group used by the production
	// reconciliation loop, without introducing an unbounded loop in the fixture.
	f.c.wg.Add(1)
	go func() { defer f.c.wg.Done(); f.c.reconcile() }()
	p05WaitSignal(t, entered, "blocked read-only reconciliation")
	var installs atomic.Int32
	if _, err := p05Replacement(f, &installs); !errors.Is(err, domain.ErrBusy) || installs.Load() != 0 {
		t.Fatalf("scenario during reconciliation = %v installs=%d", err, installs.Load())
	}
	first, second, secondStarted := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() { f.c.Close(); close(first) }()
	p05WaitSignal(t, canceled, "reconciliation cancellation")
	go func() { close(secondStarted); f.c.Close(); close(second) }()
	<-secondStarted
	select {
	case <-first:
		t.Fatal("Close returned before canceled reconciliation joined")
	case <-second:
		t.Fatal("second Close returned before canceled reconciliation joined")
	case <-time.After(80 * time.Millisecond):
	}
	once.Do(func() { close(release) })
	p05WaitSignal(t, first, "first Close join")
	p05WaitSignal(t, second, "second Close join")
	if f.writes.Load() != 1 {
		t.Fatalf("read-only reconciliation replayed writer: %d", f.writes.Load())
	}
}

func TestP05ScenarioRejectsPendingSettlement(t *testing.T) {
	f := newRepairFixtureMode(t, domain.ModeRehearsal)
	p := f.proposal(t)
	p05WaitIdle(t, f.c)
	inc, _ := f.c.Find(p.IncidentID)
	f.c.mu.Lock()
	f.c.pending[inc.ID] = inc
	f.c.mu.Unlock()
	var installs atomic.Int32
	if _, err := p05Replacement(f, &installs); !errors.Is(err, domain.ErrBusy) || installs.Load() != 0 {
		t.Fatalf("scenario during pending settlement = %v installs=%d", err, installs.Load())
	}
	if got := f.c.ActiveSession(); got == nil || !got.Active {
		t.Fatal("pending settlement rejection stopped session")
	}
}

func TestP05ScenarioRejectsClosedCoordinator(t *testing.T) {
	f := newRepairFixtureMode(t, domain.ModeRehearsal)
	f.c.Close()
	var installs atomic.Int32
	if _, err := p05Replacement(f, &installs); err == nil || installs.Load() != 0 {
		t.Fatalf("closed scenario = %v installs=%d", err, installs.Load())
	}
	settings, err := f.st.LoadSettings(context.Background())
	if err != nil || settings.Revision != 1 {
		t.Fatalf("closed scenario changed settings: %+v %v", settings, err)
	}
}

func TestP05ScenarioRollbackPreservesSessionAndProposal(t *testing.T) {
	for _, table := range []string{"app_settings", "monitoring_sessions", "proposals"} {
		t.Run(table, func(t *testing.T) {
			f := newRepairFixtureMode(t, domain.ModeRehearsal)
			p := f.proposal(t)
			p05WaitIdle(t, f.c)
			beforeSession, beforeIncidents := f.c.Snapshot()
			beforeProposals, beforeActions := f.c.RepairSnapshot()
			beforeSettings, err := f.st.LoadSettings(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			beforeIncident, err := f.st.GetIncident(context.Background(), p.IncidentID)
			if err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("sqlite", f.st.Path())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec("CREATE TRIGGER fail_p05_scenario BEFORE UPDATE ON " + table + " BEGIN SELECT RAISE(ABORT,'fixture scenario failure'); END"); err != nil {
				t.Fatal(err)
			}
			var installs atomic.Int32
			if _, err := p05Replacement(f, &installs); err == nil {
				t.Fatal("scenario ignored injected persistence failure")
			}
			afterSession, afterIncidents := f.c.Snapshot()
			afterProposals, afterActions := f.c.RepairSnapshot()
			if installs.Load() != 0 || !reflect.DeepEqual(beforeSession, afterSession) || !reflect.DeepEqual(beforeIncidents, afterIncidents) || !reflect.DeepEqual(beforeProposals, afterProposals) || !reflect.DeepEqual(beforeActions, afterActions) {
				t.Fatalf("failed transaction changed cache/install: installs=%d", installs.Load())
			}
			f.mu.Lock()
			cachedSettings := f.settings
			f.mu.Unlock()
			durableSettings, err := f.st.LoadSettings(context.Background())
			if err != nil || !reflect.DeepEqual(beforeSettings, durableSettings) || !reflect.DeepEqual(beforeSettings, cachedSettings) {
				t.Fatalf("failed transaction changed settings: %+v %v", durableSettings, err)
			}
			durableSession, err := f.st.ActiveMonitoringSession(context.Background())
			if err != nil || !reflect.DeepEqual(beforeSession, durableSession) {
				t.Fatalf("failed transaction changed durable session: %+v %v", durableSession, err)
			}
			durableIncident, err := f.st.GetIncident(context.Background(), p.IncidentID)
			if err != nil || !reflect.DeepEqual(beforeIncident, durableIncident) {
				t.Fatalf("failed transaction changed durable incident: %+v %v", durableIncident, err)
			}
			durableProposals, durableActions, err := f.st.LoadRepairs(context.Background())
			if err != nil || !reflect.DeepEqual(beforeProposals, durableProposals) || len(durableActions) != 0 {
				t.Fatalf("failed transaction changed durable repairs: %+v %+v %v", durableProposals, durableActions, err)
			}
			if _, err := db.Exec("DROP TRIGGER fail_p05_scenario"); err != nil {
				t.Fatal(err)
			}
			updated, err := p05Replacement(f, &installs)
			if err != nil || updated.Revision != beforeSettings.Revision+1 || installs.Load() != 1 {
				t.Fatalf("retry failed: %+v %v installs=%d", updated, err, installs.Load())
			}
			if got := f.c.ActiveSession(); got == nil || got.Active || got.ID != beforeSession.ID {
				t.Fatalf("successful replacement did not stop original session: %+v", got)
			}
			if got, err := f.st.ActiveMonitoringSession(context.Background()); err != nil || got != nil {
				t.Fatalf("successful replacement left durable active session: %+v %v", got, err)
			}
			ps, _ := f.c.RepairSnapshot()
			durableProposals, _, err = f.st.LoadRepairs(context.Background())
			if err != nil || len(ps) != 1 || ps[0].Status != domain.ProposalInvalidated || !reflect.DeepEqual(ps, durableProposals) {
				t.Fatalf("successful replacement did not invalidate cached/durable proposal: %+v %+v %v", ps, durableProposals, err)
			}
			if got, _ := f.c.Find(p.IncidentID); got.Stage != domain.StageInterrupted || got.RepairAvailable {
				t.Fatalf("successful replacement retained old incident action: %+v", got)
			}
			stopped := *f.c.ActiveSession().StoppedAt
			f.clock.Add(int64(time.Second))
			if _, err := p05Replacement(f, &installs); err != nil {
				t.Fatal(err)
			}
			if got := f.c.ActiveSession(); got.StoppedAt == nil || !got.StoppedAt.Equal(stopped) {
				t.Fatal("idle scenario replacement rewrote original stop time")
			}
		})
	}
}
