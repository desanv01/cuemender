package obs

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

var ErrRepairPrecondition = errors.New("OBS repair precondition changed")

// Only this package's transport can implement the write primitive. Keeping it
// separate preserves Backend's read-only discovery/diagnostic surface.
type trackWriter interface {
	setInputAudioTracks(context.Context, domain.InputRef, map[string]bool) error
}

type trackOperation struct {
	backend Backend
	gen     uint64
	rev     int64
	ctx     context.Context
	worker  context.Context
	cancel  func()
	manager *Manager
}

func (m *Manager) admitTrackOperation(ctx context.Context, request domain.TrackStateRequest) (*trackOperation, error) {
	if ctx.Err() != nil || request.Input.Name == "" || request.Input.Context == "" {
		return nil, ErrRepairPrecondition
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.backend == nil || m.conn.Status != domain.ConnectionConnected ||
		m.gen != request.Generation || m.configRevision != request.ConfigRevision ||
		(request.Input.UUID == "" && m.needsRevalidation) {
		return nil, ErrRepairPrecondition
	}
	select {
	case m.operations <- struct{}{}:
	default:
		return nil, ErrBusy
	}
	workerCtx := m.workerCtx
	if workerCtx == nil {
		workerCtx = m.ctx
	}
	opCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(workerCtx, cancel)
	m.workerWG.Add(1)
	return &trackOperation{backend: m.backend, gen: m.gen, rev: m.configRevision, ctx: opCtx, worker: workerCtx,
		manager: m, cancel: func() { stop(); cancel(); <-m.operations; m.workerWG.Done() }}, nil
}

func (op *trackOperation) transportError(err error) {
	if err != nil && !errors.Is(err, ErrRepairPrecondition) && !errors.Is(err, ErrBusy) {
		// The adapter closes a canceled/timed-out SDK transport. Invalidate
		// caches immediately, then let the existing lifecycle reconnect for
		// reads. Never retry this operation or replay an uncertain write.
		op.manager.handleTransportLoss(op.worker, op.gen, op.backend)
	}
}

func (op *trackOperation) current() bool {
	m := op.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	return op.ctx.Err() == nil && !m.closed && m.backend == op.backend &&
		m.gen == op.gen && m.configRevision == op.rev && m.conn.Status == domain.ConnectionConnected
}

func (op *trackOperation) configured(request domain.TrackRepairRequest) bool {
	m := op.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	mic, track := m.settings.Microphone, m.settings.StreamTrack
	return mic != nil && mic.Name == request.Input.Name &&
		(mic.UUID == "" || mic.UUID == request.Input.UUID) &&
		(mic.Context == "" || mic.Context == request.Input.Context) &&
		track != nil && *track == request.SelectedTrack && m.settings.TrackConfirmedAt != nil &&
		(request.Input.UUID != "" || !m.needsRevalidation)
}

// readState brackets identity and tracks with collection reads, then resolves
// identity again: a UUID-only track RPC must not hide a concurrent rename.
func (op *trackOperation) readState(ctx context.Context, ref domain.InputRef) (domain.TrackState, error) {
	state := domain.TrackState{Generation: op.gen, ConfigRevision: op.rev}
	reader, ok := op.backend.(interface {
		SceneCollection(context.Context) (string, error)
	})
	if !ok || !op.current() {
		return state, ErrRepairPrecondition
	}
	collection, err := reader.SceneCollection(ctx)
	if err != nil {
		return state, err
	}
	if collection == "" || collection != ref.Context {
		return state, ErrRepairPrecondition
	}
	resolve := func() (domain.InputRef, error) {
		inputs, err := op.backend.Inputs(ctx)
		if err != nil {
			return domain.InputRef{}, err
		}
		matches := 0
		var resolved domain.InputRef
		for _, input := range inputs {
			if input.Name == ref.Name && (ref.UUID == "" || input.UUID == ref.UUID) {
				matches++
				resolved = domain.InputRef{Name: input.Name, UUID: input.UUID, Context: collection}
			}
		}
		if matches != 1 {
			return domain.InputRef{}, ErrRepairPrecondition
		}
		return resolved, nil
	}
	resolved, err := resolve()
	if err != nil {
		return state, err
	}
	tracks, err := op.backend.InputAudioTracks(ctx, resolved)
	if err != nil {
		return state, err
	}
	if !exactTracks(tracks) {
		return state, ErrRepairPrecondition
	}
	again, err := resolve()
	if err != nil {
		return state, err
	}
	if resolved != again {
		return state, ErrRepairPrecondition
	}
	collectionAfter, err := reader.SceneCollection(ctx)
	if err != nil {
		return state, err
	}
	if collectionAfter != collection || !op.current() || ctx.Err() != nil {
		return state, ErrRepairPrecondition
	}
	state.Input, state.Tracks, state.ObservedAt = resolved, cloneTracks(tracks), op.manager.now().UTC()
	return state, nil
}

// ReadTrackState is fresh, strict and read-only. It is also the reconciliation
// seam; its source is the supplied original identity, not current expectations.
func (m *Manager) ReadTrackState(ctx context.Context, request domain.TrackStateRequest) (domain.TrackState, error) {
	op, err := m.admitTrackOperation(ctx, request)
	if err != nil {
		return domain.TrackState{}, err
	}
	defer op.cancel()
	readCtx, cancel := context.WithTimeout(op.ctx, 5*time.Second)
	defer cancel()
	state, err := op.readState(readCtx, request.Input)
	op.transportError(err)
	return state, err
}

// RepairTrack performs the sole supported change. beforeDispatch must atomically
// revalidate application authority and commit intent before returning nil. No
// manager mutex is held through that callback or network I/O. The callback must
// not reenter the manager lifecycle (Disconnect/Close join this owned operation).
// Its successful return claims dispatch; subsequent stop cannot erase the audit.
func (m *Manager) RepairTrack(ctx context.Context, request domain.TrackRepairRequest, beforeDispatch func(domain.TrackRepairPreflight) error) domain.TrackRepairResult {
	result := domain.TrackRepairResult{ActionID: request.ActionID, Generation: request.Generation,
		ConfigRevision: request.ConfigRevision, Status: domain.TrackRepairInvalidated, DetailCode: "precondition_changed"}
	if request.ActionID == "" || request.SelectedTrack < 1 || request.SelectedTrack > 6 ||
		!exactTracks(request.ExpectedTracks) || beforeDispatch == nil {
		return result
	}
	request.ExpectedTracks = cloneTracks(request.ExpectedTracks)
	op, err := m.admitTrackOperation(ctx, domain.TrackStateRequest{Input: request.Input,
		Generation: request.Generation, ConfigRevision: request.ConfigRevision})
	if err != nil {
		if errors.Is(err, ErrBusy) {
			result.DetailCode = "obs_busy"
		}
		return result
	}
	defer op.cancel()
	writer, ok := op.backend.(trackWriter)
	if !ok || !op.configured(request) {
		return result
	}
	preflightCtx, cancel := context.WithTimeout(op.ctx, 5*time.Second)
	state, err := op.readState(preflightCtx, request.Input)
	cancel()
	op.transportError(err)
	if err != nil || !op.configured(request) || !op.current() {
		return result
	}
	key := fmt.Sprintf("track%d", request.SelectedTrack)
	desired := cloneTracks(request.ExpectedTracks)
	desired[key] = true
	preflight := domain.TrackRepairPreflight{ActionID: request.ActionID, State: state,
		DesiredTracks: desired, SelectedTrack: request.SelectedTrack}
	result.Preflight = clonePreflight(preflight)
	if reflect.DeepEqual(state.Tracks, desired) {
		result.Status, result.DetailCode = domain.TrackRepairNoOp, "track_already_assigned"
		result.Observed = &state
		return result
	}
	if !reflect.DeepEqual(state.Tracks, request.ExpectedTracks) {
		result.DetailCode = "track_map_changed"
		return result
	}
	if err := beforeDispatch(*clonePreflight(preflight)); err != nil {
		result.DetailCode = "dispatch_rejected"
		return result
	}
	result.DispatchAdmitted = true
	if !op.current() || !op.configured(request) {
		return result
	}
	// This is conservative: the SDK may reject before a socket write, but
	// after invocation the adapter cannot prove that OBS did not apply it.
	result.PossiblyTransmitted = true
	result.Status, result.DetailCode = domain.TrackRepairUnknown, "write_outcome_unknown"
	if err := writer.setInputAudioTracks(op.ctx, state.Input, desired); err != nil {
		op.transportError(err)
		return result
	}
	if !op.current() {
		return result
	}
	readbackCtx, cancel := context.WithTimeout(op.ctx, 5*time.Second)
	observed, err := op.readState(readbackCtx, state.Input)
	cancel()
	op.transportError(err)
	if err != nil || !op.current() {
		result.DetailCode = "readback_unavailable"
		return result
	}
	result.Observed = &observed
	if reflect.DeepEqual(observed.Tracks, desired) {
		result.Status, result.DetailCode = domain.TrackRepairVerified, "track_map_verified"
	} else {
		result.Status, result.DetailCode = domain.TrackRepairFailed, "readback_mismatch"
	}
	return result
}

func exactTracks(tracks map[string]bool) bool { return len(tracks) == 6 && completeTracks(tracks) }

func cloneTracks(tracks map[string]bool) map[string]bool {
	out := make(map[string]bool, len(tracks))
	for key, value := range tracks {
		out[key] = value
	}
	return out
}

func clonePreflight(in domain.TrackRepairPreflight) *domain.TrackRepairPreflight {
	in.State.Tracks = cloneTracks(in.State.Tracks)
	in.DesiredTracks = cloneTracks(in.DesiredTracks)
	return &in
}
