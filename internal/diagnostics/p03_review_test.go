//go:build reviewrepro

package diagnostics

import (
	"github.com/desanv01/cuemender/internal/domain"
	"testing"
	"time"
)

func TestReviewP03RejectUnresolvedMicrophone(t *testing.T) {
	for _, change := range []string{"missing", "wrong_identity", "unknown_context", "changed_context", "unconfirmed_track"} {
		t.Run(change, func(t *testing.T) {
			ev := baseEvidence(time.Now())
			muted := true
			ev.Observation.Mute = &muted
			ev.Observation.Tracks["track2"] = false
			switch change {
			case "missing":
				ev.Discovery.Inputs = nil
			case "wrong_identity":
				ev.Observation.Microphone = &domain.InputRef{Name: "Other", UUID: "other"}
			case "unknown_context":
				ev.Discovery.CollectionKnown = false
			case "changed_context":
				ev.Discovery.SceneCollection = "Other"
				ev.Observation.SceneCollection = "Other"
			case "unconfirmed_track":
				ev.Settings.TrackConfirmedAt = nil
			}
			for _, f := range Evaluate(ev).Findings {
				if f.Category == "audio" && (change != "unconfirmed_track" || f.Code == domain.FindingMICTrackMissing) {
					t.Fatalf("unsupported audio finding for %s: %s", change, f.Code)
				}
			}
		})
	}
}

func TestReviewP03WrongSceneIsLimitation(t *testing.T) {
	ev := baseEvidence(time.Now())
	ev.Observation.ProgramScene = "Other"
	ev.Observation.ProgramSceneUUID = "other"
	for _, check := range Evaluate(ev).Checks {
		if check.Category == "video" && check.Status == "ok" {
			t.Fatal("wrong program scene reported sufficient/healthy instead of limitation")
		}
	}
}
