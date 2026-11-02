package incidents

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/desanv01/cuemender/internal/actions"
	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/store"
)

// WithSettingsChange puts the durable settings commit and its cache installation
// on the same authority boundary as session transitions and dispatch admission.
// commit must not perform provider I/O or call back into coordinator admission.
func (c *Coordinator) WithSettingsChange(ctx context.Context, commit func() error, install ...func()) error {
	if err := c.acquire(ctx); err != nil {
		return err
	}
	defer c.release()
	if err := commit(); err != nil {
		return err
	}
	c.invalidateLocked(ctx, "settings_changed")
	for _, publish := range install {
		publish()
	}
	return nil
}

func (c *Coordinator) incidentActionPendingLocked(id string) bool {
	for _, a := range c.actions {
		if a.IncidentID == id && domain.ActionPending(a.Status) {
			return true
		}
	}
	return false
}

func cloneRepair[T any](in T) T {
	raw, _ := json.Marshal(in)
	var out T
	_ = json.Unmarshal(raw, &out)
	return out
}

func (c *Coordinator) RepairSnapshot() ([]domain.Proposal, []domain.Action) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ps := make([]domain.Proposal, 0, len(c.proposals))
	as := make([]domain.Action, 0, len(c.actions))
	for _, p := range c.proposals {
		if p.Status == domain.ProposalProposed && !c.now().Before(p.ExpiresAt) {
			p.Status = domain.ProposalExpired
		}
		ps = append(ps, cloneRepair(p))
	}
	for _, a := range c.actions {
		as = append(as, cloneRepair(a))
	}
	return ps, as
}

func (c *Coordinator) invalidateProposals(ctx context.Context, incidentID, status string) error {
	c.mu.Lock()
	var candidates []domain.Proposal
	for _, p := range c.proposals {
		if incidentID != "" && p.IncidentID != incidentID {
			continue
		}
		if p.Status != domain.ProposalProposed && p.Status != domain.ProposalReserved {
			continue
		}
		if a, ok := c.actions[p.ActionID]; ok && a.IntentAt != nil {
			continue
		}
		p.Status = status
		candidates = append(candidates, p)
	}
	c.mu.Unlock()
	for _, p := range candidates {
		inc, ok := c.Find(p.IncidentID)
		var update *domain.Incident
		if ok {
			inc.RepairAvailable = false
			if inc.Stage == domain.StageAwaitingApproval {
				inc.Stage = domain.StageFindingAvailable
			}
			inc.UpdatedAt = c.now().UTC()
			update = &inc
		}
		if err := c.store.SaveRepair(ctx, &p, nil, update, false); err != nil {
			return err
		}
		c.mu.Lock()
		c.proposals[p.ID] = p
		c.mu.Unlock()
		if update != nil {
			c.setIncident(inc)
		}
	}
	if len(candidates) > 0 {
		c.publish()
	}
	return nil
}

func (c *Coordinator) Approve(ctx context.Context, id string, revision int64) (domain.Action, error) {
	if err := c.acquire(ctx); err != nil {
		return domain.Action{}, err
	}
	defer c.release()
	c.mu.Lock()
	p, ok := c.proposals[id]
	if !ok {
		c.mu.Unlock()
		return domain.Action{}, domain.ErrProposalNotFound
	}
	if p.ActionID != "" {
		a, exists := c.actions[p.ActionID]
		c.mu.Unlock()
		if exists {
			return cloneRepair(a), nil
		}
		return domain.Action{}, domain.ErrActionConflict
	}
	busy := c.actionRunning
	closed := c.closed
	c.mu.Unlock()
	if p.Status == domain.ProposalExpired {
		return domain.Action{}, domain.ErrProposalExpired
	}
	if revision != p.ConfigRevision || p.Status != domain.ProposalProposed || closed {
		return domain.Action{}, domain.ErrActionConflict
	}
	if !c.now().Before(p.ExpiresAt) {
		_ = c.invalidateProposals(ctx, p.IncidentID, domain.ProposalExpired)
		c.publish()
		return domain.Action{}, domain.ErrProposalExpired
	}
	ready := c.status()
	if !ready.OBSConnected || !ready.TwitchConnected {
		return domain.Action{}, domain.ErrUnavailable
	}
	if !ready.ResolutionReady || ready.TwitchGeneration != p.TwitchGeneration {
		return domain.Action{}, domain.ErrActionConflict
	}
	if err := actions.Validate(p, c.ActiveSession(), c.settings(), ready.OBSGeneration, true, c.now()); err != nil {
		return domain.Action{}, err
	}
	inc, exists := c.Find(p.IncidentID)
	if !exists || inc.Stage != domain.StageAwaitingApproval || inc.SessionID != p.SessionID {
		return domain.Action{}, domain.ErrActionConflict
	}
	if busy || c.repair == nil {
		return domain.Action{}, domain.ErrJobLimit
	}
	now := c.now().UTC()
	a := domain.Action{ID: store.NewSessionID(), ProposalID: p.ID, IncidentID: p.IncidentID, Binding: cloneRepair(p), Status: domain.ActionReserved, DetailCode: "awaiting_preflight", CreatedAt: now, UpdatedAt: now}
	p.Status = domain.ProposalReserved
	p.ActionID = a.ID
	inc.Stage = domain.StageApplying
	inc.RepairAvailable = false
	inc.UpdatedAt = now
	if err := c.store.SaveRepair(ctx, &p, &a, &inc, false); err != nil {
		return domain.Action{}, err
	}
	c.mu.Lock()
	c.proposals[p.ID] = p
	c.actions[a.ID] = a
	c.actionRunning = true
	c.wg.Add(1)
	c.mu.Unlock()
	c.setIncident(inc)
	c.publish()
	go c.runAction(a)
	return cloneRepair(a), nil
}

func (c *Coordinator) admitDispatch(ctx context.Context, id string, f domain.TrackRepairPreflight) error {
	if err := c.acquire(ctx); err != nil {
		return err
	}
	defer c.release()
	c.mu.Lock()
	a, ok := c.actions[id]
	p := c.proposals[a.ProposalID]
	closed := c.closed
	c.mu.Unlock()
	if !ok || closed || a.Status != domain.ActionReserved || p.Status != domain.ProposalReserved || p.ActionID != id || f.ActionID != id || !actions.MatchesPreflight(a.Binding, f) {
		return domain.ErrActionConflict
	}
	ready := c.status()
	if !ready.ResolutionReady || !ready.TwitchConnected || ready.TwitchGeneration != a.Binding.TwitchGeneration {
		return domain.ErrActionConflict
	}
	if err := actions.Validate(a.Binding, c.ActiveSession(), c.settings(), ready.OBSGeneration, ready.OBSConnected, c.now()); err != nil {
		return err
	}
	inc, ok := c.Find(a.IncidentID)
	if !ok || inc.Stage != domain.StageApplying {
		return domain.ErrActionConflict
	}
	now := c.now().UTC()
	a.Status = domain.ActionIntent
	a.DetailCode = "dispatch_admitted"
	a.IntentAt = &now
	a.UpdatedAt = now
	a.Preflight = &f
	if err := c.store.SaveRepair(ctx, nil, &a, &inc, true); err != nil {
		return err
	}
	c.mu.Lock()
	c.actions[id] = cloneRepair(a)
	c.mu.Unlock()
	c.publish()
	if err := ctx.Err(); err != nil {
		return err
	}
	ready = c.status()
	return actions.Validate(a.Binding, c.ActiveSession(), c.settings(), ready.OBSGeneration, ready.ResolutionReady && ready.OBSConnected && ready.TwitchConnected && ready.TwitchGeneration == a.Binding.TwitchGeneration, c.now())
}

func (c *Coordinator) runAction(a domain.Action) {
	defer c.wg.Done()
	result := c.repair(c.opCtx, domain.TrackRepairRequest{ActionID: a.ID, Input: a.Binding.Input, Generation: a.Binding.Generation, ConfigRevision: a.Binding.ConfigRevision, ExpectedTracks: actions.CloneTracks(a.Binding.ExpectedTracks), SelectedTrack: a.Binding.SelectedTrack}, func(f domain.TrackRepairPreflight) error { return c.admitDispatch(c.opCtx, a.ID, f) })
	// Settlement is owned cleanup and must still run after Close cancels admission.
	c.gate <- struct{}{}
	c.mu.Lock()
	a = c.actions[a.ID]
	c.mu.Unlock()
	a.Status = result.Status
	a.DetailCode = result.DetailCode
	a.PossiblyTransmitted = result.PossiblyTransmitted
	a.UpdatedAt = c.now().UTC()
	a.Preflight = result.Preflight
	a.Observed = result.Observed
	if a.Status == domain.TrackRepairVerified {
		ready := c.status()
		if ready.OBSGeneration != result.Generation || !ready.OBSConnected {
			a.Status = domain.TrackRepairUnknown
			a.DetailCode = "generation_changed"
		}
	}
	c.settleAction(a)
	c.mu.Lock()
	c.actionRunning = false
	c.mu.Unlock()
	c.release()
	c.publish()
	c.SignalIntegrationChange()
}

func (c *Coordinator) settleAction(a domain.Action) {
	inc, ok := c.Find(a.IncidentID)
	if !ok {
		return
	}
	inc.UpdatedAt = c.now().UTC()
	inc.RepairAvailable = false
	switch a.Status {
	case domain.TrackRepairVerified:
		inc.Stage = domain.StageAwaitingFeedback
		inc.LocalVerification = domain.LocalVerified
	case domain.TrackRepairNoOp:
		inc.Stage = domain.StageAwaitingFeedback
		inc.LocalVerification = domain.LocalNotAttempted
	case domain.TrackRepairUnknown:
		inc.Stage = domain.StageActionUnknown
		inc.LocalVerification = domain.LocalUnknown
	case domain.TrackRepairFailed:
		inc.Stage = domain.StageUnresolved
		inc.LocalVerification = domain.LocalFailed
	default:
		inc.Stage = domain.StageFindingAvailable
		inc.LocalVerification = domain.LocalNotAttempted
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.store.SaveRepair(ctx, nil, &a, &inc, false); err != nil {
		// A durable unfinished intent remains recoverable. Keep the UI honest
		// when outcome persistence fails and do not allow another mutation.
		a.Status = domain.TrackRepairUnknown
		a.DetailCode = "outcome_persistence_failed"
		inc.Stage = domain.StageActionUnknown
		inc.LocalVerification = domain.LocalUnknown
	}
	c.mu.Lock()
	c.actions[a.ID] = cloneRepair(a)
	c.mu.Unlock()
	c.setIncident(inc)
}

// Signals coalesce; exactly one owned worker handles invalidation and read-only
// reconciliation. Integration callbacks never join their own adapter operation.
func (c *Coordinator) SignalIntegrationChange() {
	select {
	case c.reconcileCh <- struct{}{}:
	default:
	}
}

func (c *Coordinator) reconcileLoop() {
	defer c.wg.Done()
	for {
		select {
		case <-c.opCtx.Done():
			return
		case <-c.reconcileCh:
			c.reconcile()
		}
	}
}

func (c *Coordinator) reconcile() {
	if err := c.acquire(c.opCtx); err != nil {
		return
	}
	ready := c.status()
	settings := c.settings()
	c.mu.Lock()
	var stale []string
	for id, p := range c.proposals {
		if (p.Status == domain.ProposalProposed || p.Status == domain.ProposalReserved) && (p.Generation != ready.OBSGeneration || p.TwitchGeneration != ready.TwitchGeneration || p.ConfigRevision != settings.Revision || (ready.OBSContext != "" && ready.OBSContext != p.Input.Context) || !ready.OBSConnected || !ready.TwitchConnected) {
			stale = append(stale, id)
		}
	}
	busy := c.actionRunning
	c.mu.Unlock()
	for _, id := range stale {
		c.mu.Lock()
		p := c.proposals[id]
		c.mu.Unlock()
		_ = c.invalidateProposals(c.opCtx, p.IncidentID, domain.ProposalInvalidated)
	}
	if busy || !ready.OBSConnected {
		c.release()
		if len(stale) > 0 {
			c.publish()
		}
		return
	}
	c.mu.Lock()
	var unknown []domain.Action
	for _, a := range c.actions {
		if a.Status == domain.TrackRepairUnknown {
			unknown = append(unknown, cloneRepair(a))
		}
	}
	if len(unknown) == 0 {
		c.mu.Unlock()
		c.release()
		if len(stale) > 0 {
			c.publish()
		}
		return
	}
	c.actionRunning = true
	c.mu.Unlock()
	c.release()
	for _, a := range unknown {
		if c.opCtx.Err() != nil {
			break
		}
		ready = c.status()
		settings = c.settings()
		if !ready.OBSConnected {
			break
		}
		ctx, cancel := context.WithTimeout(c.opCtx, 5*time.Second)
		state, err := c.readTrack(ctx, domain.TrackStateRequest{Input: a.Binding.Input, Generation: ready.OBSGeneration, ConfigRevision: settings.Revision})
		cancel()
		if err != nil {
			continue
		}
		if err = c.acquire(c.opCtx); err != nil {
			break
		}
		current := c.status()
		if state.Generation == current.OBSGeneration && current.OBSConnected && state.ConfigRevision == c.settings().Revision && state.Input == a.Binding.Input {
			a.Observed = &state
			a.Reconciled = true
			a.UpdatedAt = c.now().UTC()
			if reflect.DeepEqual(state.Tracks, a.Binding.DesiredTracks) {
				a.Status = domain.TrackRepairVerified
				a.DetailCode = "current_configuration_matches"
			} else {
				a.Status = domain.TrackRepairFailed
				a.DetailCode = "current_configuration_differs"
			}
			c.settleAction(a)
		}
		c.release()
		c.publish()
	}
	c.gate <- struct{}{}
	c.mu.Lock()
	c.actionRunning = false
	c.mu.Unlock()
	c.release()
	c.publish()
}
