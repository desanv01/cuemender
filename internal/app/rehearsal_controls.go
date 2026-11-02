package app

import (
	"context"

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
	now := a.now().UTC()
	expectations.TrackConfirmedAt = &now
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	current := a.currentSettings()
	return a.coordinator.ReplaceRehearsal(ctx, current.Revision, expectations, func(updated domain.Settings, caches func()) {
		a.stateMu.Lock()
		defer a.stateMu.Unlock()
		a.rehearsal.setScenario(scenario)
		a.rehearsal.setRevision(updated.Revision)
		a.settings = updated
		caches()
	})
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
	result := a.coordinator.IssueResult(ctx, category, id+"-d", id+"-m", "rehearsal-viewer", 1, a.now().UTC(), session.ID)
	return result.Incident, result.Err
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
	command := vote
	if vote == "stillbroken" {
		command = "still_broken"
	}
	result := a.coordinator.FeedbackResult(ctx, command, publicID, "rehearsal-viewer", 1, a.now().UTC(), session.ID)
	return result.Incident, result.Err
}
