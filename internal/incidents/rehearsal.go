package incidents

import (
	"context"
	"github.com/desanv01/cuemender/internal/domain"
)

// ReplaceRehearsal shares admission with diagnosis, approval, reconciliation,
// settings, feedback and Close. install must only update in-memory state, and
// runs the supplied cache install under the application's projection lock.
func (c *Coordinator) ReplaceRehearsal(ctx context.Context, revision int64, exp domain.Expectations, install func(domain.Settings, func())) (domain.Settings, error) {
	if c.mode != domain.ModeRehearsal {
		return domain.Settings{}, domain.ErrModeConflict
	}
	if err := c.acquire(ctx); err != nil {
		return domain.Settings{}, err
	}
	defer c.release()
	now := c.now().UTC()
	c.mu.Lock()
	busy := len(c.jobs) > 0 || len(c.pending) > 0 || c.actionRunning
	for _, a := range c.actions {
		busy = busy || domain.ActionPending(a.Status)
	}
	if busy {
		c.mu.Unlock()
		return domain.Settings{}, domain.ErrBusy
	}
	sessionID := ""
	if c.session != nil {
		sessionID = c.session.ID
	}
	var changes []domain.Incident
	for _, inc := range c.incidents {
		if !domain.StageTerminal(inc.Stage) {
			inc.Stage = domain.StageInterrupted
			inc.UpdatedAt = now
			inc.RepairAvailable = false
			changes = append(changes, inc)
		}
	}
	var proposals []domain.Proposal
	for _, p := range c.proposals {
		if p.Status == domain.ProposalProposed || p.Status == domain.ProposalReserved {
			p.Status = domain.ProposalInvalidated
			proposals = append(proposals, p)
		}
	}
	c.mu.Unlock()
	settings, err := c.store.ReplaceRehearsal(ctx, revision, exp, sessionID, changes, proposals, now)
	if err != nil {
		return domain.Settings{}, err
	}
	install(settings, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.session != nil && c.session.Active {
			c.session.Active = false
			c.session.StoppedAt = &now
		}
		for _, inc := range changes {
			c.incidents[inc.ID] = inc
		}
		for _, p := range proposals {
			c.proposals[p.ID] = p
		}
	})
	c.publish()
	return settings, nil
}
