package app

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

// rehearsalState is an in-process provider used only by --mode rehearsal. It
// never constructs a socket, dialer, keyring or live sender.
type rehearsalState struct {
	mu         sync.Mutex
	scenario   string
	tracks     map[string]bool
	mute       *bool
	written    bool
	generation uint64
	revision   int64
}

func newRehearsalState(settings domain.Settings) *rehearsalState {
	return &rehearsalState{scenario: "track_missing", tracks: trackMap(false), mute: boolPtr(false), generation: 1, revision: settings.Revision}
}

type rehearsalSnapshot struct {
	scenario   string
	tracks     map[string]bool
	mute       *bool
	written    bool
	generation uint64
	revision   int64
}

func (r *rehearsalState) snapshot() rehearsalSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return rehearsalSnapshot{scenario: r.scenario, tracks: cloneTrackMap(r.tracks), mute: r.mute, written: r.written, generation: r.generation, revision: r.revision}
}

func (r *rehearsalState) restore(s rehearsalSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scenario, r.tracks, r.mute, r.written, r.generation, r.revision = s.scenario, s.tracks, s.mute, s.written, s.generation, s.revision
}

func (r *rehearsalState) Generation() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.generation
}

func boolPtr(v bool) *bool { return &v }

func trackMap(selected bool) map[string]bool {
	t := map[string]bool{"track1": true, "track2": true, "track3": true, "track4": true, "track5": true, "track6": true}
	t["track2"] = selected
	return t
}

func (r *rehearsalState) setScenario(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scenario = name
	r.written = false
	r.generation++
	switch name {
	case "mic_muted":
		r.mute = boolPtr(true)
		r.tracks = trackMap(true)
	case "track_missing", "unknown_write":
		r.mute = boolPtr(false)
		r.tracks = trackMap(false)
	case "wrong_scene":
		r.mute = boolPtr(false)
		r.tracks = trackMap(true)
	}
}

func (r *rehearsalState) setRevision(revision int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revision = revision
}

func (r *rehearsalState) Read(ctx context.Context, settings domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if settings.Revision != r.revision {
		return nil, nil, domain.ErrRevisionConflict
	}
	now := time.Now().UTC()
	discovery := &domain.OBSDiscovery{
		Generation: r.generation, ConfigRevision: settings.Revision, SceneCollection: "Rehearsal", CollectionKnown: true,
		CapturedAt: now,
		Inputs:     []domain.OBSInput{{Name: "Rehearsal Mic", UUID: "rehearsal-mic"}},
		Scenes: []domain.OBSScene{{Name: "Rehearsal Main", UUID: "rehearsal-scene", Items: []domain.OBSSceneItem{
			{ID: 1, SourceName: "Rehearsal Camera", SourceUUID: "rehearsal-camera", Enabled: true},
		}}},
	}
	scene := "Rehearsal Main"
	sceneUUID := "rehearsal-scene"
	if r.scenario == "wrong_scene" {
		scene, sceneUUID = "Rehearsal Other", "rehearsal-other"
	}
	obs := &domain.OBSObservation{
		Generation: r.generation, ConfigRevision: settings.Revision, SceneCollection: "Rehearsal", ObservedAt: now,
		Microphone: &domain.InputRef{Name: "Rehearsal Mic", UUID: "rehearsal-mic"},
		Mute:       r.mute, Tracks: cloneTrackMap(r.tracks), ProgramScene: scene, ProgramSceneUUID: sceneUUID,
		VisualItemEnabled: boolPtr(true), Streaming: boolPtr(true),
	}
	return discovery, obs, nil
}

func cloneTrackMap(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (r *rehearsalState) ReadTrack(ctx context.Context, req domain.TrackStateRequest) (domain.TrackState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.TrackState{}, err
	}
	if req.Input.Name != "Rehearsal Mic" || req.Input.UUID != "rehearsal-mic" || req.Input.Context != "Rehearsal" ||
		req.Generation != r.generation || req.ConfigRevision != r.revision {
		return domain.TrackState{}, domain.ErrActionConflict
	}
	return domain.TrackState{Input: req.Input, Tracks: cloneTrackMap(r.tracks), Generation: r.generation, ConfigRevision: r.revision, ObservedAt: time.Now().UTC()}, nil
}

func (r *rehearsalState) Repair(ctx context.Context, req domain.TrackRepairRequest, admit func(domain.TrackRepairPreflight) error) domain.TrackRepairResult {
	if ctx.Err() != nil {
		return domain.TrackRepairResult{ActionID: req.ActionID, Status: domain.TrackRepairInvalidated, DetailCode: "canceled_before_admission"}
	}
	r.mu.Lock()
	scenario := r.scenario
	actual := cloneTrackMap(r.tracks)
	generation := r.generation
	revision := r.revision
	r.mu.Unlock()
	invalid := func(code string) domain.TrackRepairResult {
		return domain.TrackRepairResult{ActionID: req.ActionID, Generation: req.Generation, ConfigRevision: req.ConfigRevision, Status: domain.TrackRepairInvalidated, DetailCode: code}
	}
	if req.Input.Name != "Rehearsal Mic" || req.Input.UUID != "rehearsal-mic" || req.Input.Context != "Rehearsal" {
		return invalid("input_identity_mismatch")
	}
	if req.Generation != generation || req.ConfigRevision != revision {
		return invalid("generation_or_revision_mismatch")
	}
	if req.SelectedTrack < 1 || req.SelectedTrack > 6 {
		return invalid("selected_track_invalid")
	}
	if len(actual) != 6 {
		return invalid("track_map_incomplete")
	}
	for track := 1; track <= 6; track++ {
		if _, ok := actual["track"+strconv.Itoa(track)]; !ok {
			return invalid("track_map_incomplete")
		}
	}
	key := "track" + strconv.Itoa(req.SelectedTrack)
	for key, value := range actual {
		if expected, ok := req.ExpectedTracks[key]; !ok || (expected != value && !(key == "track"+strconv.Itoa(req.SelectedTrack) && value)) {
			return invalid("track_map_changed")
		}
	}
	if len(req.ExpectedTracks) != 6 {
		return invalid("track_map_changed")
	}
	desired := cloneTrackMap(actual)
	desired[key] = true
	preflight := domain.TrackRepairPreflight{
		ActionID: req.ActionID, SelectedTrack: req.SelectedTrack,
		State:         domain.TrackState{Input: req.Input, Tracks: actual, Generation: req.Generation, ConfigRevision: req.ConfigRevision, ObservedAt: time.Now().UTC()},
		DesiredTracks: desired,
	}
	if actual[key] {
		return domain.TrackRepairResult{ActionID: req.ActionID, Generation: req.Generation, ConfigRevision: req.ConfigRevision, Status: domain.TrackRepairNoOp, DetailCode: "already_assigned", Preflight: &preflight, Observed: &preflight.State}
	}
	if ctx.Err() != nil {
		return invalid("canceled_before_admission")
	}
	if admit != nil {
		if err := admit(preflight); err != nil {
			return domain.TrackRepairResult{ActionID: req.ActionID, Status: domain.TrackRepairInvalidated, DetailCode: "admission_rejected", Preflight: &preflight}
		}
	}
	if ctx.Err() != nil {
		return domain.TrackRepairResult{ActionID: req.ActionID, Generation: req.Generation, ConfigRevision: req.ConfigRevision, Status: domain.TrackRepairInvalidated, DetailCode: "canceled_before_write", Preflight: &preflight}
	}
	if scenario == "unknown_write" {
		return domain.TrackRepairResult{ActionID: req.ActionID, Generation: req.Generation, ConfigRevision: req.ConfigRevision, Status: domain.TrackRepairUnknown, DetailCode: "simulated_unknown", DispatchAdmitted: true, PossiblyTransmitted: true, Preflight: &preflight}
	}
	r.mu.Lock()
	if r.generation != generation || r.revision != revision {
		r.mu.Unlock()
		return invalid("provider_replaced_before_write")
	}
	if err := ctx.Err(); err != nil {
		r.mu.Unlock()
		return invalid("canceled_before_write")
	}
	for key, value := range actual {
		if r.tracks[key] != value {
			r.mu.Unlock()
			return invalid("track_map_changed_before_write")
		}
	}
	r.tracks[key] = true
	r.written = true
	observed := domain.TrackState{Input: req.Input, Tracks: cloneTrackMap(r.tracks), Generation: req.Generation, ConfigRevision: req.ConfigRevision, ObservedAt: time.Now().UTC()}
	r.mu.Unlock()
	return domain.TrackRepairResult{ActionID: req.ActionID, Generation: req.Generation, ConfigRevision: req.ConfigRevision, Status: domain.TrackRepairVerified, DetailCode: "simulated_verified", DispatchAdmitted: true, PossiblyTransmitted: true, Preflight: &preflight, Observed: &observed}
}
