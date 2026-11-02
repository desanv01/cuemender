package actions

import (
	"github.com/desanv01/cuemender/internal/domain"
	"testing"
	"time"
)

func TestProposalRejectsAmbiguousIdentityAndMalformedEvidence(t *testing.T) {
	now := time.Now().UTC()
	track := 2
	settings := domain.Settings{Revision: 1, Expectations: domain.Expectations{Microphone: &domain.InputRef{Name: "Mic", Context: "C"}, StreamTrack: &track, TrackConfirmedAt: &now}}
	for _, kind := range []string{"duplicate", "extra_track", "wrong_context", "wrong_observed_input", "wrong_revision", "expired_evidence", "bad_track"} {
		t.Run(kind, func(t *testing.T) {
			s := settings
			copyInput := *settings.Microphone
			s.Microphone = &copyInput
			d := &domain.OBSDiscovery{Generation: 1, ConfigRevision: 1, CollectionKnown: true, SceneCollection: "C", Inputs: []domain.OBSInput{{Name: "Mic", UUID: "one"}}}
			o := &domain.OBSObservation{Generation: 1, ConfigRevision: 1, SceneCollection: "C", Microphone: s.Microphone, ObservedAt: now, Tracks: map[string]bool{"track1": true, "track2": false, "track3": false, "track4": true, "track5": false, "track6": true}}
			switch kind {
			case "duplicate":
				d.Inputs = append(d.Inputs, domain.OBSInput{Name: "Mic", UUID: "two"})
			case "extra_track":
				o.Tracks["track7"] = false
			case "wrong_context":
				d.SceneCollection = "Other"
			case "wrong_observed_input":
				o.Microphone = &domain.InputRef{Name: "Other"}
			case "wrong_revision":
				o.ConfigRevision = 2
			case "expired_evidence":
				o.ObservedAt = now.Add(-6 * time.Second)
			case "bad_track":
				bad := 7
				s.StreamTrack = &bad
			}
			if _, ok := Proposal("p", domain.Incident{}, s, d, o, now); ok {
				t.Fatal("unsafe evidence accepted")
			}
		})
	}
}
