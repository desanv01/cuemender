package incidents

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/actions"
	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/store"
)

type repairFixture struct {
	c        *Coordinator
	st       *store.Store
	settings domain.Settings
	clock    atomic.Int64
	writes   atomic.Int32
	before   chan struct{}
	resume   chan struct{}
	sent     chan struct{}
	finish   chan struct{}
	unknown  bool
	mu       sync.Mutex
}

func newRepairFixture(t *testing.T, onChange ...func(*repairFixture)) *repairFixture {
	return newRepairFixtureMode(t, domain.ModeLive, onChange...)
}

func newRepairFixtureMode(t *testing.T, mode string, onChange ...func(*repairFixture)) *repairFixture {
	t.Helper()
	f := &repairFixture{before: make(chan struct{}, 1), resume: make(chan struct{}), sent: make(chan struct{}, 1), finish: make(chan struct{})}
	f.clock.Store(time.Now().UTC().UnixNano())
	ctx := context.Background()
	var err error
	f.st, err = store.OpenMode(ctx, filepath.Join(t.TempDir(), "repair.db"), mode)
	if err != nil {
		t.Fatal(err)
	}
	now := f.now()
	track := 2
	f.settings, err = f.st.SaveSettings(ctx, 0, domain.Expectations{Microphone: &domain.InputRef{Name: "Mic", UUID: "mic", Context: "Collection"}, StreamTrack: &track, TrackConfirmedAt: &now})
	if err != nil {
		t.Fatal(err)
	}
	f.c, err = NewChecked(Options{Mode: mode, Store: f.st, Now: f.now, OnChange: func() {
		for _, changed := range onChange {
			changed(f)
		}
	}, Settings: func() domain.Settings { f.mu.Lock(); defer f.mu.Unlock(); return f.settings }, Status: func() Readiness {
		return Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, OBSGeneration: 1, ResolutionReady: true}
	},
		Read: func(_ context.Context, s domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
			mute := true
			return &domain.OBSDiscovery{Generation: 1, ConfigRevision: s.Revision, CollectionKnown: true, SceneCollection: "Collection", Inputs: []domain.OBSInput{{Name: "Mic", UUID: "mic"}}}, &domain.OBSObservation{Generation: 1, ConfigRevision: s.Revision, SceneCollection: "Collection", ObservedAt: f.now(), Microphone: s.Microphone, Mute: &mute, Tracks: repairTracks()}, nil
		},
		Repair: func(ctx context.Context, r domain.TrackRepairRequest, cb func(domain.TrackRepairPreflight) error) domain.TrackRepairResult {
			result := domain.TrackRepairResult{ActionID: r.ActionID, Generation: r.Generation, ConfigRevision: r.ConfigRevision, Status: domain.TrackRepairInvalidated, DetailCode: "dispatch_rejected"}
			f.before <- struct{}{}
			select {
			case <-f.resume:
			case <-ctx.Done():
				return result
			}
			desired := actions.CloneTracks(r.ExpectedTracks)
			desired["track2"] = true
			p := domain.TrackRepairPreflight{ActionID: r.ActionID, State: domain.TrackState{Input: r.Input, Tracks: r.ExpectedTracks, Generation: r.Generation, ConfigRevision: r.ConfigRevision, ObservedAt: f.now()}, DesiredTracks: desired, SelectedTrack: 2}
			result.Preflight = &p
			if cb(p) != nil {
				return result
			}
			result.DispatchAdmitted = true
			result.PossiblyTransmitted = true
			f.writes.Add(1)
			f.sent <- struct{}{}
			select {
			case <-f.finish:
			case <-ctx.Done():
				result.Status = domain.TrackRepairUnknown
				return result
			}
			if f.unknown {
				result.Status = domain.TrackRepairUnknown
				result.DetailCode = "write_outcome_unknown"
			} else {
				result.Status = domain.TrackRepairVerified
				result.DetailCode = "track_map_verified"
				result.Observed = &domain.TrackState{Input: r.Input, Tracks: desired, Generation: r.Generation, ConfigRevision: r.ConfigRevision, ObservedAt: f.now()}
			}
			return result
		}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.c.Close(); _ = f.st.Close() })
	return f
}
func (f *repairFixture) now() time.Time { return time.Unix(0, f.clock.Load()).UTC() }
func repairTracks() map[string]bool {
	return map[string]bool{"track1": true, "track2": false, "track3": false, "track4": true, "track5": false, "track6": true}
}
func (f *repairFixture) proposal(t *testing.T) domain.Proposal {
	t.Helper()
	if _, err := f.c.Start(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	inc, err := f.c.Check(context.Background(), "audio", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	waitIncident(t, f.c, inc.ID, domain.StageAwaitingApproval)
	ps, _ := f.c.RepairSnapshot()
	if len(ps) != 1 {
		t.Fatalf("proposals=%+v", ps)
	}
	return ps[0]
}
func waitAction(t *testing.T, c *Coordinator, id, status string) domain.Action {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, as := c.RepairSnapshot()
		for _, a := range as {
			if a.ID == id && a.Status == status {
				return a
			}
		}
		time.Sleep(time.Millisecond)
	}
	_, as := c.RepairSnapshot()
	t.Fatalf("action %s never %s: %+v", id, status, as)
	return domain.Action{}
}

func TestP04ConcurrentApprovalSingleDurableAction(t *testing.T) {
	f := newRepairFixture(t)
	p := f.proposal(t)
	var wg sync.WaitGroup
	ids := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, err := f.c.Approve(context.Background(), p.ID, 1)
			if err != nil {
				t.Errorf("approve: %v", err)
				return
			}
			ids <- a.ID
		}()
	}
	wg.Wait()
	close(ids)
	var id string
	for got := range ids {
		if id != "" && got != id {
			t.Fatal("duplicate action")
		}
		id = got
	}
	close(f.resume)
	close(f.finish)
	a := waitAction(t, f.c, id, domain.TrackRepairVerified)
	if f.writes.Load() != 1 || a.IntentAt == nil {
		t.Fatalf("writes=%d action=%+v", f.writes.Load(), a)
	}
	_, as, err := f.st.LoadRepairs(context.Background())
	if err != nil || len(as) != 1 {
		t.Fatalf("durable actions=%d err=%v", len(as), err)
	}
	inc, _ := f.c.Find(p.IncidentID)
	if len(inc.Findings) != 2 || inc.LocalVerification != domain.LocalVerified {
		t.Fatalf("remaining findings lost: %+v", inc)
	}
}

func TestP04ExpiredAndStaleApprovalNeverRunAdapter(t *testing.T) {
	for _, kind := range []string{"expiry", "revision", "mode", "session", "generation", "dismiss"} {
		t.Run(kind, func(t *testing.T) {
			f := newRepairFixture(t)
			p := f.proposal(t)
			revision := int64(1)
			switch kind {
			case "expiry":
				f.clock.Add(int64(actions.ProposalTTL))
			case "revision":
				revision = 2
			case "mode":
				f.c.mu.Lock()
				p.Mode = domain.ModeRehearsal
				f.c.proposals[p.ID] = p
				f.c.mu.Unlock()
			case "session":
				_ = f.c.Stop(context.Background())
			case "generation":
				f.c.status = func() Readiness {
					return Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, OBSGeneration: 2}
				}
			case "dismiss":
				if _, err := f.c.Dismiss(context.Background(), p.IncidentID); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.c.Approve(context.Background(), p.ID, revision); err == nil {
				t.Fatal("stale approval accepted")
			}
			if f.writes.Load() != 0 {
				t.Fatal("write")
			}
			select {
			case <-f.before:
				t.Fatal("adapter ran")
			default:
			}
		})
	}
}

func TestP04StopAndSettingsWinBeforeDispatch(t *testing.T) {
	for _, kind := range []string{"stop", "settings"} {
		t.Run(kind, func(t *testing.T) {
			f := newRepairFixture(t)
			p := f.proposal(t)
			a, err := f.c.Approve(context.Background(), p.ID, 1)
			if err != nil {
				t.Fatal(err)
			}
			<-f.before
			if kind == "stop" {
				if err = f.c.Stop(context.Background()); err != nil {
					t.Fatal(err)
				}
			} else {
				err = f.c.WithSettingsChange(context.Background(), func() error {
					f.mu.Lock()
					defer f.mu.Unlock()
					var e error
					f.settings, e = f.st.SaveSettings(context.Background(), 1, f.settings.Expectations)
					return e
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			close(f.resume)
			waitAction(t, f.c, a.ID, domain.TrackRepairInvalidated)
			if f.writes.Load() != 0 {
				t.Fatal("stale write")
			}
		})
	}
}

func TestP04IntentFailurePreventsWrite(t *testing.T) {
	f := newRepairFixture(t)
	p := f.proposal(t)
	db, err := sql.Open("sqlite", f.st.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER fail_p04_intent BEFORE UPDATE ON actions WHEN NEW.status='intent_recorded' BEGIN SELECT RAISE(ABORT,'fixture intent failure'); END`); err != nil {
		t.Fatal(err)
	}
	a, err := f.c.Approve(context.Background(), p.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	close(f.resume)
	waitAction(t, f.c, a.ID, domain.TrackRepairInvalidated)
	if f.writes.Load() != 0 {
		t.Fatal("write after failed durable intent")
	}
	_, as, _ := f.st.LoadRepairs(context.Background())
	if as[0].IntentAt != nil {
		t.Fatal("failed transaction committed intent")
	}
}

func TestP04ExpiryAtDurableIntentBoundaryPreventsWrite(t *testing.T) {
	f := newRepairFixture(t, func(f *repairFixture) {
		_, as := f.c.RepairSnapshot()
		for _, a := range as {
			if a.Status == domain.ActionIntent {
				f.clock.Store(a.Binding.ExpiresAt.UnixNano())
			}
		}
	})
	p := f.proposal(t)
	a, err := f.c.Approve(context.Background(), p.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	close(f.resume)
	settled := waitAction(t, f.c, a.ID, domain.TrackRepairInvalidated)
	if f.writes.Load() != 0 || settled.IntentAt == nil {
		t.Fatalf("writes=%d intent=%v", f.writes.Load(), settled.IntentAt)
	}
}

func TestP04StopAfterDispatchRetainsAuditAndRejectsDismissRecheck(t *testing.T) {
	f := newRepairFixture(t)
	p := f.proposal(t)
	a, err := f.c.Approve(context.Background(), p.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	close(f.resume)
	<-f.sent
	if _, err = f.c.Dismiss(context.Background(), p.IncidentID); !errors.Is(err, domain.ErrActionConflict) {
		t.Fatalf("dismiss=%v", err)
	}
	if _, err = f.c.Check(context.Background(), "audio", 1, p.IncidentID); !errors.Is(err, domain.ErrActionConflict) {
		t.Fatalf("recheck=%v", err)
	}
	if err = f.c.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	inc, _ := f.c.Find(p.IncidentID)
	if inc.Stage != domain.StageApplying {
		t.Fatalf("stop erased write: %+v", inc)
	}
	close(f.finish)
	waitAction(t, f.c, a.ID, domain.TrackRepairVerified)
	if f.writes.Load() != 1 {
		t.Fatal("write repeated")
	}
	active, public, _ := f.c.PublicSnapshot()
	if active || len(public) > 0 {
		t.Fatal("old stopped session leaked")
	}
}

func TestP04UnknownReadOnlyReconciliation(t *testing.T) {
	f := newRepairFixture(t)
	f.unknown = true
	p := f.proposal(t)
	a, err := f.c.Approve(context.Background(), p.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	close(f.resume)
	close(f.finish)
	waitAction(t, f.c, a.ID, domain.TrackRepairUnknown)
	var reads atomic.Int32
	f.c.readTrack = func(_ context.Context, r domain.TrackStateRequest) (domain.TrackState, error) {
		reads.Add(1)
		desired := repairTracks()
		desired["track2"] = true
		return domain.TrackState{Input: r.Input, Generation: r.Generation, ConfigRevision: r.ConfigRevision, Tracks: desired, ObservedAt: f.now()}, nil
	}
	f.c.reconcile()
	settled := waitAction(t, f.c, a.ID, domain.TrackRepairVerified)
	if !settled.Reconciled || settled.DetailCode != "current_configuration_matches" || reads.Load() != 1 || f.writes.Load() != 1 {
		t.Fatalf("bad reconciliation: %+v reads=%d writes=%d", settled, reads.Load(), f.writes.Load())
	}
	if again, err := f.c.Approve(context.Background(), p.ID, 1); err != nil || again.ID != a.ID || f.writes.Load() != 1 {
		t.Fatal("repeat replayed unknown")
	}
}

func TestP04RestartIntentReconcilesOriginalSourceWithoutWriter(t *testing.T) {
	f := newRepairFixture(t)
	p := f.proposal(t)
	// Seed the durable crash boundary, without running any external action.
	inc, _ := f.c.Find(p.IncidentID)
	now := f.now()
	p.Status = domain.ProposalReserved
	p.ActionID = "crashed-action"
	a := domain.Action{ID: p.ActionID, ProposalID: p.ID, IncidentID: p.IncidentID, Binding: p, Status: domain.ActionIntent, IntentAt: &now, CreatedAt: now, UpdatedAt: now}
	inc.Stage = domain.StageApplying
	if err := f.st.SaveRepair(context.Background(), &p, &a, &inc, false); err != nil {
		t.Fatal(err)
	}
	f.c.Close()
	// New expectations select another source; recovery must read the saved mic.
	newTrack := 3
	settings, err := f.st.SaveSettings(context.Background(), 1, domain.Expectations{Microphone: &domain.InputRef{Name: "Other", UUID: "other", Context: "Collection"}, StreamTrack: &newTrack, TrackConfirmedAt: &now})
	if err != nil {
		t.Fatal(err)
	}
	var reads, writes atomic.Int32
	recovered, err := NewChecked(Options{Store: f.st, Now: f.now, Settings: func() domain.Settings { return settings }, Status: func() Readiness { return Readiness{OBSConnected: true, OBSGeneration: 7} }, ReadTrack: func(_ context.Context, r domain.TrackStateRequest) (domain.TrackState, error) {
		reads.Add(1)
		if r.Input != a.Binding.Input || r.Generation != 7 || r.ConfigRevision != settings.Revision {
			t.Errorf("retargeted recovery: %+v", r)
		}
		return domain.TrackState{Input: r.Input, Tracks: a.Binding.DesiredTracks, Generation: r.Generation, ConfigRevision: r.ConfigRevision, ObservedAt: f.now()}, nil
	}, Repair: func(context.Context, domain.TrackRepairRequest, func(domain.TrackRepairPreflight) error) domain.TrackRepairResult {
		writes.Add(1)
		return domain.TrackRepairResult{}
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	_, as := recovered.RepairSnapshot()
	if len(as) != 1 || as[0].Status != domain.TrackRepairUnknown {
		t.Fatalf("restart did not mark unknown: %+v", as)
	}
	if _, err = recovered.Approve(context.Background(), p.ID, 1); err != nil {
		t.Fatalf("repeat must return action without execution: %v", err)
	}
	recovered.SignalIntegrationChange()
	got := waitAction(t, recovered, a.ID, domain.TrackRepairVerified)
	if !got.Reconciled || reads.Load() != 1 || writes.Load() != 0 || recovered.ActiveSession() != nil {
		t.Fatalf("recovery=%+v reads=%d writes=%d", got, reads.Load(), writes.Load())
	}
}

func TestP04EmptyReconciliationCannotReleaseActionOwner(t *testing.T) {
	f := newRepairFixture(t)
	p := f.proposal(t)
	// First exercise the empty-unknown path, then hold a live reservation while
	// repeated reconciliation notifications arrive. Only its owner may clear it.
	f.c.reconcile()
	a, err := f.c.Approve(context.Background(), p.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	<-f.before
	for i := 0; i < 20; i++ {
		f.c.reconcile()
		f.c.mu.Lock()
		running := f.c.actionRunning
		f.c.mu.Unlock()
		if !running {
			t.Fatal("reconciliation released live action ownership")
		}
	}
	close(f.resume)
	close(f.finish)
	waitAction(t, f.c, a.ID, domain.TrackRepairVerified)
}

func TestP04OutcomePersistenceFailureRetriesReadOnly(t *testing.T) {
	f := newRepairFixture(t)
	p := f.proposal(t)
	db, err := sql.Open("sqlite", f.st.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER fail_p04_outcome BEFORE UPDATE ON actions WHEN NEW.status='verified' BEGIN SELECT RAISE(ABORT,'fixture outcome failure'); END`); err != nil {
		t.Fatal(err)
	}
	a, err := f.c.Approve(context.Background(), p.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	close(f.resume)
	close(f.finish)
	failed := waitAction(t, f.c, a.ID, domain.TrackRepairUnknown)
	if failed.DetailCode != "outcome_persistence_failed" || f.writes.Load() != 1 {
		t.Fatalf("failure=%+v writes=%d", failed, f.writes.Load())
	}
	_, durable, err := f.st.LoadRepairs(context.Background())
	if err != nil || durable[0].Status != domain.ActionIntent {
		t.Fatalf("intent was lost: %+v %v", durable, err)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_p04_outcome`); err != nil {
		t.Fatal(err)
	}
	// Stop reloads durable records; it must retain runtime uncertainty rather
	// than turn it back into an unreconciled intent until process restart.
	if err := f.c.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitAction(t, f.c, a.ID, domain.TrackRepairUnknown)
	f.c.readTrack = func(_ context.Context, r domain.TrackStateRequest) (domain.TrackState, error) {
		return domain.TrackState{Input: r.Input, Tracks: a.Binding.DesiredTracks, Generation: r.Generation, ConfigRevision: r.ConfigRevision, ObservedAt: f.now()}, nil
	}
	f.c.reconcile()
	settled := waitAction(t, f.c, a.ID, domain.TrackRepairVerified)
	if !settled.Reconciled || f.writes.Load() != 1 {
		t.Fatalf("retry wrote again: %+v writes=%d", settled, f.writes.Load())
	}
	_, durable, err = f.st.LoadRepairs(context.Background())
	if err != nil || durable[0].Status != domain.TrackRepairVerified {
		t.Fatal("read-only retry not persisted")
	}
}

func TestP04RetentionPreservesUnknownAndPrunesRepairCache(t *testing.T) {
	f := newRepairFixture(t)
	p := f.proposal(t)
	inc, _ := f.c.Find(p.IncidentID)
	old := f.now().Add(-31 * 24 * time.Hour)
	p.Status = domain.ProposalReserved
	p.ActionID = "retention-action"
	a := domain.Action{ID: p.ActionID, ProposalID: p.ID, IncidentID: p.IncidentID, Binding: p, Status: domain.TrackRepairUnknown, IntentAt: &old, CreatedAt: old, UpdatedAt: old}
	inc.Stage = domain.StageClosed
	inc.UpdatedAt = old
	inc.ClosedAt = &old
	if err := f.st.SaveRepair(context.Background(), &p, &a, &inc, false); err != nil {
		t.Fatal(err)
	}
	if err := f.st.PurgeCompletedIncidents(context.Background(), f.now()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.GetIncident(context.Background(), inc.ID); err != nil {
		t.Fatal("unknown action audit purged")
	}
	a.Status = domain.TrackRepairVerified
	if err := f.st.SaveRepair(context.Background(), nil, &a, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := f.st.PurgeCompletedIncidents(context.Background(), f.now()); err != nil {
		t.Fatal(err)
	}
	if err := f.c.reload(); err != nil {
		t.Fatal(err)
	}
	ps, as := f.c.RepairSnapshot()
	if len(ps) != 0 || len(as) != 0 {
		t.Fatalf("ghost repair cache: proposals=%d actions=%d", len(ps), len(as))
	}
}

// Discovery refresh briefly makes resolution unknown until the next coherent
// observation. It must not consume an otherwise unexpired approval forever.
func TestP04DiscoveryRefreshPreservesProposalButContextChangeInvalidates(t *testing.T) {
	f := newRepairFixture(t)
	p := f.proposal(t)
	f.c.status = func() Readiness {
		return Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, OBSGeneration: 1, OBSContext: "Collection", ResolutionReady: false}
	}
	f.c.reconcile()
	ps, _ := f.c.RepairSnapshot()
	if len(ps) != 1 || ps[0].Status != domain.ProposalProposed {
		t.Fatalf("transient refresh discarded approval: %+v", ps)
	}
	if _, err := f.c.Approve(context.Background(), p.ID, 1); err == nil {
		t.Fatal("approval admitted while resolution unavailable")
	}
	f.c.status = func() Readiness {
		return Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, OBSGeneration: 1, OBSContext: "Different collection", ResolutionReady: true}
	}
	f.c.reconcile()
	ps, _ = f.c.RepairSnapshot()
	if len(ps) != 1 || ps[0].Status != domain.ProposalInvalidated {
		t.Fatalf("context replacement retained approval: %+v", ps)
	}
	if f.writes.Load() != 0 {
		t.Fatal("unexpected write")
	}
}
