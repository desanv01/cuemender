package incidents

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/desanv01/cuemender/internal/diagnostics"
	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/store"
)

const (
	maxJobs    = 2
	jobTimeout = 5 * time.Second
)

type Readiness struct {
	TwitchConnected  bool
	OBSConnected     bool
	ResolutionReady  bool
	OBSGeneration    uint64
	TwitchGeneration uint64
}

type Options struct {
	Store    *store.Store
	Read     func(context.Context, domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error)
	Settings func() domain.Settings
	Status   func() Readiness
	Reply    func(string) bool
	OnChange func()
	Now      func() time.Time
}

type job struct {
	ctx        context.Context
	cancel     context.CancelFunc
	incident   domain.Incident
	settings   domain.Settings
	requestID  string
	generation uint64
	settled    bool
}

type Coordinator struct {
	store    *store.Store
	read     func(context.Context, domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error)
	settings func() domain.Settings
	status   func() Readiness
	reply    func(string) bool
	onChange func()
	now      func() time.Time

	mu        sync.Mutex
	gate      chan struct{}
	opCtx     context.Context
	opCancel  context.CancelFunc
	pending   map[string]domain.Incident
	session   *domain.MonitoringSessionInfo
	jobs      map[string]*job
	incidents map[string]domain.Incident
	order     []string
	closed    bool
	closeOnce sync.Once
	wg        sync.WaitGroup
}

func New(opts Options) *Coordinator {
	c, _ := NewChecked(opts)
	return c
}

func NewChecked(opts Options) (*Coordinator, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	c := &Coordinator{
		store: opts.Store, read: opts.Read, settings: opts.Settings, status: opts.Status,
		reply: opts.Reply, onChange: opts.OnChange, now: opts.Now,
		jobs: map[string]*job{}, incidents: map[string]domain.Incident{},
		pending: map[string]domain.Incident{},
		gate:    make(chan struct{}, 1),
	}
	c.opCtx, c.opCancel = context.WithCancel(context.Background())
	if err := c.restart(); err != nil {
		c.opCancel()
		return nil, err
	}
	return c, nil
}

func (c *Coordinator) restart() error {
	if c.store == nil {
		return nil
	}
	ctx := context.Background()
	if err := c.store.StopAllMonitoringSessions(ctx, c.now()); err != nil {
		return err
	}
	if err := c.store.PurgeCompletedIncidents(ctx, c.now()); err != nil {
		return err
	}
	if err := c.store.InterruptOpenIncidents(ctx, c.now()); err != nil {
		return err
	}
	list, err := c.store.ListRetainedIncidents(ctx)
	if err != nil {
		return err
	}
	for _, inc := range list {
		c.incidents[inc.ID] = inc
		c.order = append(c.order, inc.ID)
	}
	return nil
}

func (c *Coordinator) acquire(ctx context.Context) error {
	var opDone <-chan struct{}
	if c.opCtx != nil {
		opDone = c.opCtx.Done()
		select {
		case <-opDone:
			return context.Canceled
		default:
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-opDone:
		return context.Canceled
	case c.gate <- struct{}{}:
		// A ready gate and cancellation can win the select together. Do not
		// admit a canceled operation just because the gate happened to be free.
		if err := ctx.Err(); err != nil {
			c.release()
			return err
		}
		if c.opCtx != nil && c.opCtx.Err() != nil {
			c.release()
			return c.opCtx.Err()
		}
		return nil
	}
}

func (c *Coordinator) release() { <-c.gate }

func (c *Coordinator) Start(ctx context.Context, expectedRevision int64) (domain.MonitoringSessionInfo, error) {
	if err := c.acquire(ctx); err != nil {
		return domain.MonitoringSessionInfo{}, err
	}
	defer c.release()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return domain.MonitoringSessionInfo{}, domain.ErrUnavailable
	}
	c.mu.Unlock()
	settings := c.settings()
	if settings.Revision != expectedRevision {
		return domain.MonitoringSessionInfo{}, domain.ErrRevisionConflict
	}
	c.mu.Lock()
	if c.session != nil && c.session.Active {
		s := *c.session
		c.mu.Unlock()
		return s, nil
	}
	c.mu.Unlock()
	ready := c.status()
	if !ready.TwitchConnected || !ready.OBSConnected {
		return domain.MonitoringSessionInfo{}, domain.ErrUnavailable
	}
	if !ready.ResolutionReady {
		return domain.MonitoringSessionInfo{}, domain.ErrNotReady
	}
	if settings.Microphone == nil && settings.Visual == nil {
		return domain.MonitoringSessionInfo{}, domain.ErrNotReady
	}
	if settings.Microphone != nil && (settings.StreamTrack == nil || settings.TrackConfirmedAt == nil) {
		return domain.MonitoringSessionInfo{}, domain.ErrNotReady
	}
	id := store.NewSessionID()
	now := c.now().UTC()
	if err := c.store.StartMonitoringSession(ctx, id, now); err != nil {
		return domain.MonitoringSessionInfo{}, err
	}
	c.mu.Lock()
	c.session = &domain.MonitoringSessionInfo{ID: id, Active: true, StartedAt: now}
	s := *c.session
	c.mu.Unlock()
	c.publish()
	return s, nil
}

func (c *Coordinator) Stop(ctx context.Context) error {
	if err := c.acquire(ctx); err != nil {
		return err
	}
	defer c.release()
	c.mu.Lock()
	pending := make(map[string]domain.Incident, len(c.pending))
	for id, inc := range c.pending {
		pending[id] = inc
	}
	active := c.session != nil && c.session.Active
	sessionID := ""
	if active {
		sessionID = c.session.ID
	}
	jobs := make([]*job, 0, len(c.jobs))
	for _, job := range c.jobs {
		jobs = append(jobs, job)
	}
	c.mu.Unlock()
	for id, observed := range pending {
		now := c.now().UTC()
		updated, changed, err := c.store.InterruptIncident(ctx, id, observed.UpdatedAt, now)
		if err != nil {
			return err
		}
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		if changed {
			c.installSettled(updated)
		}
	}
	if !active {
		if len(jobs) == 0 {
			if len(pending) > 0 {
				c.publish()
			}
			return nil
		}
		for _, job := range jobs {
			job.cancel()
			inc, ok := c.Find(job.incident.ID)
			if !ok || domain.StageTerminal(inc.Stage) {
				continue
			}
			observed := inc
			inc.Stage = domain.StageInterrupted
			inc.UpdatedAt = c.now().UTC()
			if err := c.store.SaveIncident(ctx, inc); err != nil {
				c.mu.Lock()
				c.pending[inc.ID] = observed
				c.mu.Unlock()
				return err
			}
			c.setIncident(inc)
		}
		c.publish()
		return nil
	}
	now := c.now().UTC()
	if err := c.store.StopMonitoringSession(ctx, sessionID, now); err != nil {
		return err
	}
	if err := c.store.InterruptOpenIncidents(ctx, now); err != nil {
		return err
	}
	_ = c.store.PurgeCompletedIncidents(ctx, now)
	c.mu.Lock()
	if c.session != nil && c.session.ID == sessionID {
		c.session.Active = false
		c.session.StoppedAt = &now
	}
	for _, job := range jobs {
		job.cancel()
	}
	c.mu.Unlock()
	if err := c.reload(); err != nil {
		return err
	}
	c.publish()
	return nil
}

func (c *Coordinator) Close() {
	c.closeOnce.Do(func() {
		if c.opCancel != nil {
			c.opCancel()
		}
		c.gate <- struct{}{}
		c.mu.Lock()
		c.closed = true
		for _, job := range c.jobs {
			job.cancel()
		}
		c.mu.Unlock()
		// Final settlement also owns the gate. Release before joining workers;
		// admission remains closed through opCtx and closed.
		c.release()
		c.wg.Wait()
	})
}

func (c *Coordinator) ActiveSession() *domain.MonitoringSessionInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil
	}
	copy := *c.session
	return &copy
}

func (c *Coordinator) Snapshot() (*domain.MonitoringSessionInfo, []domain.Incident) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var sess *domain.MonitoringSessionInfo
	if c.session != nil {
		copy := *c.session
		sess = &copy
	}
	out := make([]domain.Incident, 0, len(c.order))
	for _, id := range c.order {
		if inc, ok := c.incidents[id]; ok {
			out = append(out, copyIncident(inc))
		}
	}
	return sess, out
}

func (c *Coordinator) Find(id string) (domain.Incident, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	inc, ok := c.incidents[id]
	return copyIncident(inc), ok
}

func copyIncident(inc domain.Incident) domain.Incident {
	out := inc
	out.Findings = append([]domain.Finding(nil), inc.Findings...)
	out.Checks = append([]domain.DiagnosticCheck(nil), inc.Checks...)
	return out
}

func (c *Coordinator) reload() error {
	if c.store == nil {
		return nil
	}
	list, err := c.store.ListRetainedIncidents(context.Background())
	if err != nil {
		return err
	}
	incidents := make(map[string]domain.Incident, len(list))
	order := make([]string, 0, len(list))
	for _, inc := range list {
		incidents[inc.ID] = inc
		order = append(order, inc.ID)
	}
	c.mu.Lock()
	c.incidents = incidents
	c.order = order
	c.mu.Unlock()
	return nil
}

func (c *Coordinator) publish() {
	if c.onChange != nil {
		c.onChange()
	}
}

func (c *Coordinator) setIncident(inc domain.Incident) {
	c.mu.Lock()
	if _, exists := c.incidents[inc.ID]; !exists {
		c.order = append(c.order, inc.ID)
	}
	c.incidents[inc.ID] = inc
	c.mu.Unlock()
}

// Check schedules one manual or join/recheck diagnostic job. It returns after
// the incident is durably in investigating state, not after the OBS read.
func (c *Coordinator) Check(ctx context.Context, category string, expectedRevision int64, incidentID string) (domain.Incident, error) {
	if err := c.acquire(ctx); err != nil {
		return domain.Incident{}, err
	}
	defer c.release()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return domain.Incident{}, domain.ErrUnavailable
	}
	c.mu.Unlock()
	if category != "audio" && category != "video" {
		return domain.Incident{}, domain.ErrInvalidInput
	}
	settings := c.settings()
	if settings.Revision != expectedRevision {
		return domain.Incident{}, domain.ErrRevisionConflict
	}
	c.mu.Lock()
	session := c.session
	if incidentID != "" {
		inc, ok := c.incidents[incidentID]
		if !ok {
			c.mu.Unlock()
			return domain.Incident{}, domain.ErrIncidentNotFound
		}
		if inc.Category != category {
			c.mu.Unlock()
			return domain.Incident{}, domain.ErrInvalidInput
		}
		if domain.StageTerminal(inc.Stage) {
			c.mu.Unlock()
			return domain.Incident{}, domain.ErrTerminalIncident
		}
		if session == nil || !session.Active || inc.SessionID != session.ID {
			c.mu.Unlock()
			return domain.Incident{}, domain.ErrTerminalIncident
		}
		if existing := c.jobs[category]; existing != nil {
			if existing.incident.ID == inc.ID {
				c.mu.Unlock()
				return inc, nil
			}
			c.mu.Unlock()
			return domain.Incident{}, domain.ErrJobLimit
		}
		inc.Stage = domain.StageInvestigating
		inc.UpdatedAt = c.now().UTC()
		incCopy := copyIncident(inc)
		c.mu.Unlock()
		if err := c.store.SaveIncident(ctx, incCopy); err != nil {
			return domain.Incident{}, err
		}
		c.setIncident(incCopy)
		c.publish()
		c.schedule(incCopy, settings)
		return incCopy, nil
	}

	if session != nil && session.Active {
		active, ok := c.incidentsByCategoryLocked(session.ID, category)
		if ok {
			if existing := c.jobs[category]; existing != nil {
				c.mu.Unlock()
				return active, nil
			}
			active.Stage = domain.StageInvestigating
			active.UpdatedAt = c.now().UTC()
			c.mu.Unlock()
			if err := c.store.SaveIncident(ctx, active); err != nil {
				return domain.Incident{}, err
			}
			c.setIncident(active)
			c.publish()
			c.schedule(active, settings)
			return active, nil
		}
	}
	if len(c.jobs) >= maxJobs {
		c.mu.Unlock()
		return domain.Incident{}, domain.ErrJobLimit
	}
	if existing := c.jobs[category]; existing != nil {
		c.mu.Unlock()
		return existing.incident, nil
	}
	manualSession := "manual-" + store.NewSessionID()
	if session != nil && session.Active {
		manualSession = session.ID
	}
	c.mu.Unlock()
	id, publicID := store.NewIncidentIDs()
	inc, err := c.store.CreateManualIncident(ctx, manualSession, category, id, publicID, c.now().UTC())
	if err != nil {
		return domain.Incident{}, err
	}
	inc.Stage = domain.StageInvestigating
	inc.UpdatedAt = c.now().UTC()
	if err := c.store.SaveIncident(ctx, inc); err != nil {
		return domain.Incident{}, err
	}
	c.setIncident(inc)
	c.publish()
	c.schedule(inc, settings)
	return inc, nil
}

func (c *Coordinator) incidentsByCategoryLocked(sessionID, category string) (domain.Incident, bool) {
	for _, id := range c.order {
		inc := c.incidents[id]
		if inc.SessionID == sessionID && inc.Category == category && !domain.StageTerminal(inc.Stage) {
			return copyIncident(inc), true
		}
	}
	return domain.Incident{}, false
}

// HandleIssue accepts one viewer report after durable dedup/grouping. It returns
// the fixed acknowledgment only after the commit.
func (c *Coordinator) HandleIssue(ctx context.Context, category, deliveryID, messageID, actorID string, generation uint64) string {
	return c.HandleIssueAt(ctx, category, deliveryID, messageID, actorID, generation, time.Time{})
}

func (c *Coordinator) HandleIssueAt(ctx context.Context, category, deliveryID, messageID, actorID string, generation uint64, receivedAt time.Time) string {
	return c.handleIssue(ctx, category, deliveryID, messageID, actorID, generation, receivedAt, "*")
}

func (c *Coordinator) HandleIssueInSessionAt(ctx context.Context, category, deliveryID, messageID, actorID string, generation uint64, receivedAt time.Time, sessionID string) string {
	return c.handleIssue(ctx, category, deliveryID, messageID, actorID, generation, receivedAt, sessionID)
}

func (c *Coordinator) handleIssue(ctx context.Context, category, deliveryID, messageID, actorID string, generation uint64, receivedAt time.Time, expectedSession string) string {
	if err := c.acquire(ctx); err != nil {
		return ""
	}
	defer c.release()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ""
	}
	c.mu.Unlock()
	settings := c.settings()
	c.mu.Lock()
	session := c.session
	if session == nil || !session.Active || session.ID == "" {
		c.mu.Unlock()
		return "CueMender monitoring is not active. Help: !streamhelp"
	}
	if expectedSession != "*" && (expectedSession == "" || expectedSession != session.ID) {
		c.mu.Unlock()
		return ""
	}
	if !receivedAt.IsZero() && receivedAt.Before(session.StartedAt) {
		c.mu.Unlock()
		return ""
	}
	if len(c.jobs) >= maxJobs && c.jobs[category] == nil {
		c.mu.Unlock()
		return "CueMender diagnostics are busy. Please retry shortly."
	}
	c.mu.Unlock()
	ready := c.status()
	if !ready.TwitchConnected || ready.TwitchGeneration == 0 || generation != ready.TwitchGeneration {
		return ""
	}
	incidentID, publicID := store.NewIncidentIDs()
	inc, accepted, err := c.store.AcceptReport(ctx, session.ID, category, deliveryID, messageID, actorID, incidentID, publicID, c.now().UTC())
	if err != nil {
		if errors.Is(err, store.ErrReportBound) {
			return "CueMender cannot accept more distinct reporters for this incident."
		}
		return ""
	}
	if !accepted {
		return ""
	}
	c.setIncident(inc)
	c.publish()
	scheduled := inc.Stage == domain.StageReported || inc.Stage == domain.StageInvestigating
	if scheduled {
		c.schedule(inc, settings)
		return fmt.Sprintf("CueMender received your %s report for %s. Diagnostics are running.", category, inc.PublicID)
	}
	return fmt.Sprintf("CueMender added your %s report to existing incident %s.", category, inc.PublicID)
}

func (c *Coordinator) HandleFeedback(ctx context.Context, command, code, actorID string) string {
	return c.HandleFeedbackAt(ctx, command, code, actorID, 0, time.Time{})
}

func (c *Coordinator) HandleFeedbackAt(ctx context.Context, command, code, actorID string, generation uint64, receivedAt time.Time) string {
	return c.handleFeedback(ctx, command, code, actorID, generation, receivedAt, "*")
}

func (c *Coordinator) HandleFeedbackInSessionAt(ctx context.Context, command, code, actorID string, generation uint64, receivedAt time.Time, sessionID string) string {
	return c.handleFeedback(ctx, command, code, actorID, generation, receivedAt, sessionID)
}

func (c *Coordinator) handleFeedback(ctx context.Context, command, code, actorID string, generation uint64, receivedAt time.Time, expectedSession string) string {
	if err := c.acquire(ctx); err != nil {
		return ""
	}
	defer c.release()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ""
	}
	c.mu.Unlock()
	c.mu.Lock()
	session := c.session
	if session == nil || !session.Active {
		c.mu.Unlock()
		return "CueMender feedback is only accepted during an active monitoring session."
	}
	var target domain.Incident
	found := false
	for _, inc := range c.incidents {
		if inc.PublicID == code {
			target = inc
			found = true
			break
		}
	}
	c.mu.Unlock()
	if !found || target.SessionID != session.ID || (expectedSession != "*" && expectedSession != session.ID) || domain.StageTerminal(target.Stage) {
		return "CueMender ignored unknown or stale feedback."
	}
	ready := c.status()
	if !ready.TwitchConnected || ready.TwitchGeneration == 0 || generation != ready.TwitchGeneration {
		return ""
	}
	if !receivedAt.IsZero() && receivedAt.Before(session.StartedAt) {
		return "CueMender ignored feedback from before this monitoring session."
	}
	if target.Stage != domain.StageAwaitingFeedback && target.Stage != domain.StageUnresolved {
		return "CueMender ignored feedback before an eligible finding."
	}
	vote := "positive"
	if command == "still_broken" {
		vote = "negative"
	}
	updated, err := c.store.AddFeedback(ctx, target.ID, actorID, vote, c.now().UTC())
	if err != nil {
		return ""
	}
	c.setIncident(updated)
	c.publish()
	return "CueMender recorded your feedback."
}

func (c *Coordinator) Dismiss(ctx context.Context, incidentID string) (domain.Incident, error) {
	if err := c.acquire(ctx); err != nil {
		return domain.Incident{}, err
	}
	defer c.release()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return domain.Incident{}, domain.ErrUnavailable
	}
	inc, ok := c.incidents[incidentID]
	if !ok {
		c.mu.Unlock()
		return domain.Incident{}, domain.ErrIncidentNotFound
	}
	if domain.StageTerminal(inc.Stage) {
		c.mu.Unlock()
		if inc.Stage == domain.StageDismissed {
			return inc, nil
		}
		return domain.Incident{}, domain.ErrTerminalIncident
	}
	if job := c.jobs[inc.Category]; job != nil && job.incident.ID == incidentID {
		job.cancel()
	}
	c.mu.Unlock()
	out, err := c.store.DismissIncident(ctx, incidentID, c.now().UTC())
	if err != nil {
		return domain.Incident{}, err
	}
	c.setIncident(out)
	_ = c.store.PurgeCompletedIncidents(ctx, c.now().UTC())
	if err := c.reload(); err != nil {
		return out, err
	}
	c.publish()
	return out, nil
}

func (c *Coordinator) sessionIDLocked() string {
	if c.session == nil {
		return ""
	}
	return c.session.ID
}

func (c *Coordinator) schedule(inc domain.Incident, settings domain.Settings) {
	ready := c.status()
	c.mu.Lock()
	if c.closed || domain.StageTerminal(inc.Stage) ||
		(inc.Stage != domain.StageReported && inc.Stage != domain.StageInvestigating) {
		c.mu.Unlock()
		return
	}
	if _, exists := c.jobs[inc.Category]; exists {
		c.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	request := &job{ctx: ctx, cancel: cancel, incident: inc, settings: settings, requestID: store.NewSessionID(), generation: ready.OBSGeneration}
	c.jobs[inc.Category] = request
	c.wg.Add(1)
	c.mu.Unlock()
	go c.run(request)
}

func (c *Coordinator) run(request *job) {
	defer c.wg.Done()
	defer request.cancel()
	defer c.finishJob(request)
	discovery, observation, readErr := c.read(request.ctx, request.settings)
	result := diagnostics.Result{}
	if readErr == nil {
		result = diagnostics.EvaluateFor(diagnostics.Evidence{
			Settings: request.settings, Discovery: discovery, Observation: observation, Now: c.now(),
		}, request.incident.Category)
	}
	if err := c.acquire(request.ctx); err != nil {
		return
	}
	defer c.release()
	currentRevision := c.settings().Revision
	ready := c.status()
	c.mu.Lock()
	if c.closed || c.jobs[request.incident.Category] != request || request.ctx.Err() != nil {
		c.mu.Unlock()
		return
	}
	if currentRevision != request.settings.Revision || ready.OBSGeneration != request.generation {
		c.mu.Unlock()
		return
	}
	if inc, ok := c.incidents[request.incident.ID]; ok && inc.SessionID == c.sessionIDLocked() {
		if c.session == nil || !c.session.Active || !ready.OBSConnected {
			c.mu.Unlock()
			return
		}
	}
	current, ok := c.incidents[request.incident.ID]
	if !ok || domain.StageTerminal(current.Stage) {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	now := c.now().UTC()
	current.UpdatedAt = now
	current.Checks = result.Checks
	current.Findings = make([]domain.Finding, 0, len(result.Findings))
	for i, finding := range result.Findings {
		finding.ID = fmt.Sprintf("%s-f%d", current.ID, i+1)
		finding.IncidentID = current.ID
		current.Findings = append(current.Findings, finding)
	}
	if readErr != nil {
		current.Stage = domain.StageInsufficientInfo
		current.Checks = []domain.DiagnosticCheck{{Name: "obs_evidence", Category: current.Category, Status: "unknown", Explanation: "obs_unavailable", Unavailable: "Fresh OBS evidence is unavailable."}}
	} else if len(result.Findings) > 0 {
		current.Stage = domain.StageFindingAvailable
		hasTrackMismatch := false
		for _, finding := range current.Findings {
			if finding.SupportedActionKind == domain.ActionAddMicToTrack {
				hasTrackMismatch = true
				break
			}
		}
		// C09: P03 track-mismatch incidents remain finding_available with the
		// supported action as descriptive metadata only; manual-only findings
		// enter awaiting_feedback.
		if !hasTrackMismatch {
			current.Stage = domain.StageAwaitingFeedback
		}
	} else if result.Sufficient {
		current.Stage = domain.StageNoSupportedFinding
	} else {
		current.Stage = domain.StageInsufficientInfo
	}
	if len(current.Findings) > 0 {
		current.EvidenceAt = &now
	}
	if observation != nil {
		current.Streaming = observation.Streaming
	}
	current.RepairAvailable = false
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 3*time.Second)
	err := c.store.SaveIncident(saveCtx, current)
	saveCancel()
	if err != nil {
		c.publish()
		return
	}
	c.setIncident(current)
	request.settled = true
	c.publish()
}

func (c *Coordinator) installSettled(inc domain.Incident) {
	c.mu.Lock()
	if current, ok := c.incidents[inc.ID]; ok && (current.UpdatedAt.After(inc.UpdatedAt) ||
		(current.UpdatedAt.Equal(inc.UpdatedAt) && domain.StageTerminal(current.Stage))) {
		c.mu.Unlock()
		return
	}
	c.incidents[inc.ID] = inc
	c.mu.Unlock()
}

func (c *Coordinator) finishJob(request *job) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// Cleanup is allowed after admission closes, but shares the operation gate
	// with dismiss/recheck through durable write, cache installation and publish.
	select {
	case c.gate <- struct{}{}:
		defer c.release()
	case <-ctx.Done():
		c.mu.Lock()
		if c.jobs[request.incident.Category] == request {
			if inc, ok := c.incidents[request.incident.ID]; !request.settled && ok &&
				(inc.Stage == domain.StageReported || inc.Stage == domain.StageInvestigating) {
				c.pending[inc.ID] = inc
			}
			delete(c.jobs, request.incident.Category)
		}
		c.mu.Unlock()
		c.publish()
		return
	}
	var observed *domain.Incident
	c.mu.Lock()
	if !request.settled && c.jobs[request.incident.Category] == request {
		if inc, ok := c.incidents[request.incident.ID]; ok && (inc.Stage == domain.StageReported || inc.Stage == domain.StageInvestigating) {
			observed = &inc
		}
	}
	c.mu.Unlock()
	if observed != nil {
		updated, ok, err := c.store.InterruptIncident(ctx, observed.ID, observed.UpdatedAt, c.now().UTC())
		if err != nil {
			c.mu.Lock()
			c.pending[observed.ID] = *observed
			c.mu.Unlock()
		} else if ok {
			c.installSettled(updated)
		}
	}
	c.mu.Lock()
	if c.jobs[request.incident.Category] == request {
		delete(c.jobs, request.incident.Category)
	}
	c.mu.Unlock()
	c.publish()
}

func (c *Coordinator) Invalidate(ctx context.Context, reason string) {
	if err := c.acquire(ctx); err != nil {
		return
	}
	defer c.release()
	c.mu.Lock()
	jobs := make([]*job, 0, len(c.jobs))
	for _, job := range c.jobs {
		jobs = append(jobs, job)
	}
	c.mu.Unlock()
	if len(jobs) == 0 {
		return
	}
	for _, job := range jobs {
		job.cancel()
		inc := job.incident
		if current, ok := c.Find(inc.ID); ok {
			inc = current
		}
		if !domain.StageTerminal(inc.Stage) {
			inc.Stage = domain.StageInterrupted
			inc.UpdatedAt = c.now().UTC()
			if err := c.store.SaveIncident(ctx, inc); err == nil {
				c.setIncident(inc)
			}
		}
	}
	c.publish()
}

func (c *Coordinator) ActiveJobs() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.jobs)
}

// PublicIncidents returns only current-session incidents with fixed messages.
func (c *Coordinator) PublicSnapshot() (active bool, incidents []domain.PublicIncident, jobs int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil || !c.session.Active {
		return false, []domain.PublicIncident{}, len(c.jobs)
	}
	out := []domain.PublicIncident{}
	for _, id := range c.order {
		inc := c.incidents[id]
		if inc.SessionID != c.session.ID {
			continue
		}
		out = append(out, domain.PublicIncident{
			PublicID: inc.PublicID, Category: inc.Category, Stage: inc.Stage,
			MessageCode: publicMessage(inc), LocalVerification: inc.LocalVerification,
			FeedbackSummary: fmt.Sprintf("%d positive / %d negative", inc.Feedback.Positive, inc.Feedback.Negative),
		})
	}
	return true, out, len(c.jobs)
}

// PublicIncidents returns only current-session incidents with fixed messages.
func (c *Coordinator) PublicIncidents() []domain.PublicIncident {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil || !c.session.Active {
		return []domain.PublicIncident{}
	}
	out := []domain.PublicIncident{}
	for _, id := range c.order {
		inc := c.incidents[id]
		if inc.SessionID != c.session.ID {
			continue
		}
		out = append(out, domain.PublicIncident{
			PublicID: inc.PublicID, Category: inc.Category, Stage: inc.Stage,
			MessageCode: publicMessage(inc), LocalVerification: inc.LocalVerification,
			FeedbackSummary: fmt.Sprintf("%d positive / %d negative", inc.Feedback.Positive, inc.Feedback.Negative),
		})
	}
	return out
}

func publicMessage(inc domain.Incident) string {
	switch inc.Stage {
	case domain.StageNoSupportedFinding:
		return "no_supported_conflict"
	case domain.StageInsufficientInfo:
		return "more_information_needed"
	case domain.StageFindingAvailable, domain.StageAwaitingApproval, domain.StageAwaitingFeedback:
		return "finding_available"
	case domain.StageInterrupted:
		return "interrupted"
	default:
		return "checking"
	}
}
