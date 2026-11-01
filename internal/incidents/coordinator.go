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
	TwitchConnected bool
	OBSConnected    bool
	ResolutionReady bool
	OBSGeneration   uint64
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
	session   *domain.MonitoringSessionInfo
	jobs      map[string]*job
	incidents map[string]domain.Incident
	order     []string
	closed    bool
	wg        sync.WaitGroup
}

func New(opts Options) *Coordinator {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	c := &Coordinator{
		store: opts.Store, read: opts.Read, settings: opts.Settings, status: opts.Status,
		reply: opts.Reply, onChange: opts.OnChange, now: opts.Now,
		jobs: map[string]*job{}, incidents: map[string]domain.Incident{},
	}
	c.restart()
	return c
}

func (c *Coordinator) restart() {
	if c.store == nil {
		return
	}
	ctx := context.Background()
	_ = c.store.StopAllMonitoringSessions(ctx, c.now())
	_ = c.store.PurgeCompletedIncidents(ctx, c.now())
	for _, inc := range c.mustList(ctx) {
		if !domain.StageTerminal(inc.Stage) {
			inc.Stage = domain.StageInterrupted
			inc.UpdatedAt = c.now()
			_ = c.store.SaveIncident(ctx, inc)
		}
	}
	for _, inc := range c.mustList(ctx) {
		c.incidents[inc.ID] = inc
		c.order = append(c.order, inc.ID)
	}
}

func (c *Coordinator) mustList(ctx context.Context) []domain.Incident {
	list, err := c.store.ListIncidents(ctx, 500)
	if err != nil {
		return nil
	}
	return list
}

func (c *Coordinator) Start(ctx context.Context, expectedRevision int64) (domain.MonitoringSessionInfo, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return domain.MonitoringSessionInfo{}, domain.ErrUnavailable
	}
	if c.session != nil && c.session.Active {
		s := *c.session
		c.mu.Unlock()
		return s, nil
	}
	c.mu.Unlock()
	settings := c.settings()
	if settings.Revision != expectedRevision {
		return domain.MonitoringSessionInfo{}, domain.ErrRevisionConflict
	}
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
	c.mu.Lock()
	if c.session == nil || !c.session.Active {
		c.mu.Unlock()
		return nil
	}
	id := c.session.ID
	now := c.now().UTC()
	c.session.Active = false
	c.session.StoppedAt = &now
	for _, job := range c.jobs {
		job.cancel()
	}
	c.jobs = map[string]*job{}
	c.mu.Unlock()
	if err := c.store.StopMonitoringSession(ctx, id, now); err != nil {
		return err
	}
	if err := c.store.InterruptOpenIncidents(ctx, now); err != nil {
		return err
	}
	c.reload()
	c.publish()
	return nil
}

func (c *Coordinator) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	for _, job := range c.jobs {
		job.cancel()
	}
	c.jobs = map[string]*job{}
	c.mu.Unlock()
	c.wg.Wait()
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

func (c *Coordinator) reload() {
	if c.store == nil {
		return
	}
	list := c.mustList(context.Background())
	c.mu.Lock()
	c.incidents = map[string]domain.Incident{}
	c.order = c.order[:0]
	for _, inc := range list {
		c.incidents[inc.ID] = inc
		c.order = append(c.order, inc.ID)
	}
	c.mu.Unlock()
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
	settings := c.settings()
	c.mu.Lock()
	session := c.session
	if session == nil || !session.Active || session.ID == "" {
		c.mu.Unlock()
		return "CueMender monitoring is not active. Help: !streamhelp"
	}
	if len(c.jobs) >= maxJobs && c.jobs[category] == nil {
		c.mu.Unlock()
		return "CueMender diagnostics are busy. Please retry shortly."
	}
	c.mu.Unlock()
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
	c.schedule(inc, settings)
	return fmt.Sprintf("CueMender received your %s report for %s. Diagnostics are running.", category, inc.PublicID)
}

func (c *Coordinator) HandleFeedback(ctx context.Context, command, code, actorID string) string {
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
	if !found || target.SessionID != session.ID || domain.StageTerminal(target.Stage) {
		return "CueMender ignored unknown or stale feedback."
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
	c.mu.Lock()
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
	if c.jobs[inc.Category] != nil {
		c.jobs[inc.Category].cancel()
		delete(c.jobs, inc.Category)
	}
	c.mu.Unlock()
	out, err := c.store.DismissIncident(ctx, incidentID, c.now().UTC())
	if err != nil {
		return domain.Incident{}, err
	}
	c.setIncident(out)
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
	c.mu.Lock()
	if c.closed || domain.StageTerminal(inc.Stage) {
		c.mu.Unlock()
		return
	}
	if _, exists := c.jobs[inc.Category]; exists {
		c.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	request := &job{ctx: ctx, cancel: cancel, incident: inc, settings: settings, requestID: store.NewSessionID(), generation: c.status().OBSGeneration}
	c.jobs[inc.Category] = request
	c.wg.Add(1)
	c.mu.Unlock()
	go c.run(request)
}

func (c *Coordinator) run(request *job) {
	defer c.wg.Done()
	defer request.cancel()
	discovery, observation, readErr := c.read(request.ctx, request.settings)
	result := diagnostics.Result{}
	if readErr == nil {
		result = diagnostics.Evaluate(diagnostics.Evidence{
			Settings: request.settings, Discovery: discovery, Observation: observation, Now: c.now(),
		})
	}
	currentRevision := c.settings().Revision
	ready := c.status()
	c.mu.Lock()
	if c.closed || c.jobs[request.incident.Category] != request {
		c.mu.Unlock()
		return
	}
	if currentRevision != request.settings.Revision {
		delete(c.jobs, request.incident.Category)
		c.mu.Unlock()
		return
	}
	if inc, ok := c.incidents[request.incident.ID]; ok && inc.SessionID == c.sessionIDLocked() {
		if c.session == nil || !c.session.Active || !ready.OBSConnected || ready.OBSGeneration != request.generation {
			delete(c.jobs, request.incident.Category)
			c.mu.Unlock()
			return
		}
	}
	current, ok := c.incidents[request.incident.ID]
	if !ok || domain.StageTerminal(current.Stage) {
		delete(c.jobs, request.incident.Category)
		c.mu.Unlock()
		return
	}
	delete(c.jobs, request.incident.Category)
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
	current.RepairAvailable = false
	if err := c.store.SaveIncident(context.Background(), current); err != nil {
		c.publish()
		return
	}
	c.setIncident(current)
	c.publish()
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
