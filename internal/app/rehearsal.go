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
	mu       sync.Mutex
	scenario string
	tracks   map[string]bool
	mute     *bool
	written  bool
}

func newRehearsalState(settings domain.Settings) *rehearsalState {
	return &rehearsalState{scenario: "track_missing", tracks: trackMap(false), mute: boolPtr(false)}
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

func (r *rehearsalState) Read(_ context.Context, settings domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	discovery := &domain.OBSDiscovery{
		Generation: 1, ConfigRevision: settings.Revision, SceneCollection: "Rehearsal", CollectionKnown: true,
		CapturedAt: now,
		Inputs:     []domain.OBSInput{{Name: "Rehearsal Mic", UUID: "rehearsal-mic"}},
		Scenes:     []domain.OBSScene{{Name: "Rehearsal Main", UUID: "rehearsal-scene"}},
	}
	scene := "Rehearsal Main"
	sceneUUID := "rehearsal-scene"
	if r.scenario == "wrong_scene" {
		scene, sceneUUID = "Rehearsal Other", "rehearsal-other"
	}
	obs := &domain.OBSObservation{
		Generation: 1, ConfigRevision: settings.Revision, SceneCollection: "Rehearsal", ObservedAt: now,
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

func (r *rehearsalState) ReadTrack(_ context.Context, req domain.TrackStateRequest) (domain.TrackState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return domain.TrackState{Input: req.Input, Tracks: cloneTrackMap(r.tracks), Generation: req.Generation, ConfigRevision: req.ConfigRevision, ObservedAt: time.Now().UTC()}, nil
}

func (r *rehearsalState) Repair(ctx context.Context, req domain.TrackRepairRequest, admit func(domain.TrackRepairPreflight) error) domain.TrackRepairResult {
	if ctx.Err() != nil {
		return domain.TrackRepairResult{ActionID: req.ActionID, Status: domain.TrackRepairInvalidated, DetailCode: "canceled_before_admission"}
	}
	r.mu.Lock()
	scenario := r.scenario
	actual := cloneTrackMap(r.tracks)
	r.mu.Unlock()
	invalid := func(code string) domain.TrackRepairResult {
		return domain.TrackRepairResult{ActionID: req.ActionID, Generation: req.Generation, ConfigRevision: req.ConfigRevision, Status: domain.TrackRepairInvalidated, DetailCode: code}
	}
	if req.Input.Name != "Rehearsal Mic" || req.Input.UUID != "rehearsal-mic" || req.Input.Context != "Rehearsal" {
		return invalid("input_identity_mismatch")
	}
	if req.SelectedTrack < 1 || req.SelectedTrack > 6 {
		return invalid("selected_track_invalid")
	}
	if len(actual) != 6 {
		return invalid("track_map_incomplete")
	}
	for key, value := range actual {
		if req.ExpectedTracks[key] != value {
			return invalid("track_map_changed")
		}
	}
	if len(req.ExpectedTracks) != 6 {
		return invalid("track_map_changed")
	}
	key := "track" + strconv.Itoa(req.SelectedTrack)
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
	r.tracks[key] = true
	r.written = true
	observed := domain.TrackState{Input: req.Input, Tracks: cloneTrackMap(r.tracks), Generation: req.Generation, ConfigRevision: req.ConfigRevision, ObservedAt: time.Now().UTC()}
	r.mu.Unlock()
	return domain.TrackRepairResult{ActionID: req.ActionID, Generation: req.Generation, ConfigRevision: req.ConfigRevision, Status: domain.TrackRepairVerified, DetailCode: "simulated_verified", DispatchAdmitted: true, PossiblyTransmitted: true, Preflight: &preflight, Observed: &observed}
}
