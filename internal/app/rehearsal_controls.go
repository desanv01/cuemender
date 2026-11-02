package app

import (
	"context"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/store"
)

func (a *App) RehearsalScenario(ctx context.Context, scenario string) (domain.Settings, error) {
	if a.rehearsal == nil {
		return domain.Settings{}, domain.ErrModeConflict
	}
	switch scenario {
	case "track_missing", "mic_muted", "wrong_scene", "unknown_write":
	default:
		return domain.Settings{}, domain.ErrInvalidInput
	}
	if err := a.coordinator.Stop(ctx); err != nil {
		return domain.Settings{}, err
	}
	a.coordinator.Invalidate(ctx, "scenario_change")
	a.rehearsal.setScenario(scenario)
	track := 2
	expectations := domain.Expectations{
		Microphone:  &domain.InputRef{Name: "Rehearsal Mic", UUID: "rehearsal-mic", Context: "Rehearsal"},
		StreamTrack: &track,
	}
	if scenario == "wrong_scene" {
		expectations.Visual = &domain.VisualExpectation{
			Scene: "Rehearsal Main", SceneUUID: "rehearsal-scene", SceneItemID: "1",
			Source: domain.InputRef{Name: "Rehearsal Camera", UUID: "rehearsal-camera", Context: "Rehearsal"},
		}
	}
	current := a.currentSettings()
	return a.SaveSettings(ctx, current.Revision, expectations)
}

func (a *App) RehearsalReport(ctx context.Context, category string) (domain.Incident, error) {
	if a.rehearsal == nil {
		return domain.Incident{}, domain.ErrModeConflict
	}
	if category != "audio" && category != "video" {
		return domain.Incident{}, domain.ErrInvalidInput
	}
	session := a.coordinator.ActiveSession()
	if session == nil || !session.Active {
		return domain.Incident{}, domain.ErrNotReady
	}
	id := "rehearsal-" + store.NewSessionID()
	reply := a.coordinator.HandleIssueInSessionAt(ctx, category, id+"-d", id+"-m", "rehearsal-viewer", 1, a.now().UTC(), session.ID)
	if reply == "" {
		return domain.Incident{}, domain.ErrInvalidInput
	}
	_, list := a.coordinator.Snapshot()
	if len(list) == 0 {
		return domain.Incident{}, domain.ErrUnavailable
	}
	return list[0], nil
}

func (a *App) RehearsalFeedback(ctx context.Context, publicID, vote string) (domain.Incident, error) {
	if a.rehearsal == nil {
		return domain.Incident{}, domain.ErrModeConflict
	}
	if vote != "fixed" && vote != "stillbroken" {
		return domain.Incident{}, domain.ErrInvalidInput
	}
	session := a.coordinator.ActiveSession()
	if session == nil || !session.Active {
		return domain.Incident{}, domain.ErrNotReady
	}
	_, list := a.coordinator.Snapshot()
	for _, inc := range list {
		if inc.PublicID != publicID {
			continue
		}
		reply := a.coordinator.HandleFeedbackInSessionAt(ctx, vote, publicID, "rehearsal-viewer", 1, time.Now().UTC(), session.ID)
		if reply == "" {
			return domain.Incident{}, domain.ErrTerminalIncident
		}
		got, _ := a.coordinator.Find(inc.ID)
		return got, nil
	}
	return domain.Incident{}, domain.ErrIncidentNotFound
}
