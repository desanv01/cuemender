package app

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/incidents"
	"github.com/desanv01/cuemender/internal/twitch"
)

const p03Help = "CueMender commands: !issueaudio, !issuevideo, !fixed CODE, !stillbroken CODE, !streamhelp."

func (a *App) currentSettings() domain.Settings {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.settings
}

func (a *App) monitoringReadiness() incidents.Readiness {
	if a.monitoringStatusOverride != nil {
		return a.monitoringStatusOverride()
	}
	settings := a.currentSettings()
	tw := a.twitchMgr.Snapshot()
	obs, _, _, resolution := a.obsMgr.SnapshotBundle(settings)
	return incidents.Readiness{
		TwitchConnected:  tw.Status == domain.ConnectionConnected,
		OBSConnected:     obs.Status == domain.ConnectionConnected && !obs.Stale,
		ResolutionReady:  resolution,
		OBSGeneration:    obs.Generation,
		TwitchGeneration: tw.ConnectionGeneration,
	}
}

func (a *App) onIncidentUpdate() {
	a.stateMu.Lock()
	a.stateRevision++
	privateJSON, errPrivate := json.Marshal(a.privateLocked())
	publicJSON, errPublic := json.Marshal(a.publicLocked())
	if errPrivate == nil {
		a.privateHub.publish(privateJSON)
	}
	if errPublic == nil {
		a.publicHub.publish(publicJSON)
	}
	a.stateMu.Unlock()
}

func (a *App) onTwitchCommand(event twitch.CommandEvent) {
	select {
	case a.intakeCh <- event:
	default:
		// Bounded intake: obsolete work is dropped rather than blocking EventSub.
	}
}

func (a *App) processTwitchCommand(event twitch.CommandEvent) {
	ctx := a.intakeCtx
	if ctx == nil {
		ctx = context.Background()
	}
	reply := ""
	switch event.Command.Kind {
	case twitch.CommandStreamHelp:
		reply = p03Help
	case twitch.CommandIssueAudio, twitch.CommandIssueVideo:
		if a.coordinator != nil {
			reply = a.coordinator.HandleIssueAt(ctx, event.Command.Category, event.DeliveryID, event.MessageID, event.ActorID, event.ConnectionGeneration, event.ReceivedAt)
		} else {
			reply = p03Help
		}
	case twitch.CommandFixed, twitch.CommandStillBroken:
		if a.coordinator != nil {
			reply = a.coordinator.HandleFeedbackAt(ctx, string(event.Command.Kind), event.Command.IncidentCode, event.ActorID, event.ConnectionGeneration, event.ReceivedAt)
		}
	}
	if strings.TrimSpace(reply) != "" {
		session := a.coordinator.ActiveSession()
		if session != nil && session.Active && !event.ReceivedAt.Before(session.StartedAt) {
			_ = a.twitchMgr.EnqueueReplyFor(event.ConnectionGeneration, reply)
		}
	}
}

func (a *App) StartMonitoring(ctx context.Context, expectedRevision int64) (domain.MonitoringSessionInfo, error) {
	if a.coordinator == nil {
		return domain.MonitoringSessionInfo{}, domain.ErrUnavailable
	}
	return a.coordinator.Start(ctx, expectedRevision)
}

func (a *App) StopMonitoring(ctx context.Context) error {
	if a.coordinator == nil {
		return nil
	}
	return a.coordinator.Stop(ctx)
}

func (a *App) CheckIncident(ctx context.Context, category string, expectedRevision int64, incidentID string) (domain.Incident, error) {
	if a.coordinator == nil {
		return domain.Incident{}, domain.ErrUnavailable
	}
	return a.coordinator.Check(ctx, category, expectedRevision, incidentID)
}

func (a *App) DismissIncident(ctx context.Context, incidentID string) (domain.Incident, error) {
	if a.coordinator == nil {
		return domain.Incident{}, domain.ErrUnavailable
	}
	return a.coordinator.Dismiss(ctx, incidentID)
}
