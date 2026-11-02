//go:build reviewrepro

package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

func p05R01Settled(t *testing.T, a *App, scenario string) domain.Incident {
	t.Helper()
	ctx := context.Background()
	s, err := a.RehearsalScenario(ctx, scenario)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.StartMonitoring(ctx, s.Revision); err != nil {
		t.Fatal(err)
	}
	inc, err := a.RehearsalReport(ctx, "audio")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := a.coordinator.Find(inc.ID)
		if got.Stage != domain.StageReported && got.Stage != domain.StageInvestigating && a.coordinator.ActiveJobs() == 0 {
			return got
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("diagnostic did not settle")
	return domain.Incident{}
}

func TestReviewP05R01FeedbackRejectsUnacceptedVotes(t *testing.T) {
	for _, scenario := range []string{"track_missing", "mic_muted"} {
		t.Run(scenario, func(t *testing.T) {
			a := p05ReviewApp(t)
			inc := p05R01Settled(t, a, scenario)
			if scenario == "mic_muted" {
				if err := a.StopMonitoring(context.Background()); err != nil {
					t.Fatal(err)
				}
				if _, err := a.StartMonitoring(context.Background(), a.currentSettings().Revision); err != nil {
					t.Fatal(err)
				}
			}
			got, err := a.RehearsalFeedback(context.Background(), inc.PublicID, "fixed")
			if err == nil {
				t.Fatalf("rejected vote reported success: stage=%s feedback=%+v", got.Stage, got.Feedback)
			}
		})
	}
}

func TestReviewP05R01ScenarioPublishesMatchingProvider(t *testing.T) {
	a := p05ReviewApp(t)
	ctx := context.Background()
	before, _ := a.PrivateSnapshot(ctx)
	sub := a.SubscribePrivate()
	defer sub.Cancel()
	updated, err := a.RehearsalScenario(ctx, "mic_muted")
	if err != nil {
		t.Fatal(err)
	}
	var last domain.PrivateState
	for {
		select {
		case raw := <-sub.C:
			if err := json.Unmarshal(raw, &last); err != nil {
				t.Fatal(err)
			}
		default:
			if last.Settings.Revision != updated.Revision || last.OBSObservation == nil || last.OBSObservation.Mute == nil || !*last.OBSObservation.Mute {
				t.Errorf("published scenario/settings disagree: settings=%d wanted=%d observation=%+v", last.Settings.Revision, updated.Revision, last.OBSObservation)
			}
			after, _ := a.PrivateSnapshot(ctx)
			if after.OBS.Generation <= before.OBS.Generation {
				t.Errorf("scenario replacement did not advance generation: before=%d after=%d", before.OBS.Generation, after.OBS.Generation)
			}
			return
		}
	}
}

func TestReviewP05R01ReconciliationRejectsForeignSource(t *testing.T) {
	r := newRehearsalState(domain.Settings{})
	state, err := r.ReadTrack(context.Background(), domain.TrackStateRequest{Input: domain.InputRef{Name: "Foreign Mic", UUID: "foreign", Context: "Other"}, Generation: 999, ConfigRevision: 999})
	if err == nil {
		t.Fatalf("readback relabelled fixture as caller identity/generation: %+v", state)
	}
}
