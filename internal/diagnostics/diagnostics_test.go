package diagnostics

import (
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

func baseEvidence(now time.Time) Evidence {
	track := 2
	confirmed := now
	settings := domain.Settings{Revision: 1, Expectations: domain.Expectations{
		Microphone:  &domain.InputRef{Name: "Mic", UUID: "mic", Context: "Collection"},
		StreamTrack: &track, TrackConfirmedAt: &confirmed,
		Visual: &domain.VisualExpectation{Scene: "Main", SceneUUID: "scene", SceneItemID: "7", Source: domain.InputRef{Name: "Cam", UUID: "cam", Context: "Collection"}},
	}}
	shared := map[string]bool{"track1": false, "track2": true, "track3": false, "track4": false, "track5": false, "track6": false}
	mute := false
	enabled := true
	streaming := false
	return Evidence{
		Settings: settings,
		Discovery: &domain.OBSDiscovery{Generation: 3, ConfigRevision: 1, SceneCollection: "Collection", CollectionKnown: true,
			Inputs: []domain.OBSInput{{Name: "Mic", UUID: "mic"}},
			Scenes: []domain.OBSScene{{Name: "Main", UUID: "scene", Items: []domain.OBSSceneItem{{ID: 7, SourceName: "Cam", SourceUUID: "cam", Enabled: true}}}},
		},
		Observation: &domain.OBSObservation{Generation: 3, ConfigRevision: 1, SceneCollection: "Collection", ObservedAt: now,
			Microphone: &domain.InputRef{Name: "Mic", UUID: "mic"}, Mute: &mute, Tracks: shared,
			ProgramScene: "Main", ProgramSceneUUID: "scene", VisualItemEnabled: &enabled, Streaming: &streaming},
		Now: now,
	}
}

func TestAllThreeChecks(t *testing.T) {
	now := time.Now()
	mute := true
	// Two simultaneous audio conflicts plus visual.
	ev := baseEvidence(now)
	ev.Observation.Mute = &mute
	ev.Observation.Tracks["track2"] = false
	hidden := false
	ev.Observation.VisualItemEnabled = &hidden
	result := Evaluate(ev)
	if len(result.Findings) != 3 {
		t.Fatalf("findings = %+v, want 3", result.Findings)
	}
	codes := map[string]bool{}
	for _, finding := range result.Findings {
		codes[finding.Code] = true
	}
	if !codes[domain.FindingMICMuted] || !codes[domain.FindingMICTrackMissing] || !codes[domain.FindingVisualHidden] {
		t.Fatalf("missing simultaneous findings: %+v", codes)
	}
}

func TestNoConflictRequiresSufficientEvidence(t *testing.T) {
	now := time.Now()
	result := Evaluate(baseEvidence(now))
	if len(result.Findings) != 0 || !result.Sufficient {
		t.Fatalf("clean evidence result = %+v", result)
	}
	ev := baseEvidence(now)
	ev.Observation.Tracks = map[string]bool{"track2": true}
	result = Evaluate(ev)
	if result.Sufficient || len(result.Findings) != 0 {
		t.Fatalf("incomplete tracks claimed healthy: %+v", result)
	}
}

func TestWrongSceneAndStaleEvidence(t *testing.T) {
	now := time.Now()
	ev := baseEvidence(now)
	ev.Observation.ProgramScene = "Other"
	ev.Observation.ProgramSceneUUID = "other"
	result := Evaluate(ev)
	if len(result.Findings) != 0 || result.Sufficient {
		t.Fatalf("configured item outside current scene must remain a limitation: %+v", result)
	}
	ev = baseEvidence(now)
	ev.Now = now.Add(6 * time.Second)
	result = Evaluate(ev)
	if result.Sufficient || len(result.Findings) != 0 {
		t.Fatalf("stale evidence produced a finding or healthy result: %+v", result)
	}
}

func TestRevisionAndGenerationMismatchAreUnknown(t *testing.T) {
	now := time.Now()
	ev := baseEvidence(now)
	ev.Observation.ConfigRevision = 2
	result := Evaluate(ev)
	if result.Sufficient || len(result.Findings) != 0 {
		t.Fatalf("mismatched revision treated as evidence: %+v", result)
	}
	ev = baseEvidence(now)
	ev.Discovery.Generation = 9
	result = Evaluate(ev)
	if result.Sufficient {
		t.Fatalf("mismatched generation treated as ready: %+v", result)
	}
}

func TestUnsupportedNestedVisualIsUnknown(t *testing.T) {
	now := time.Now()
	ev := baseEvidence(now)
	ev.Discovery.Scenes[0].Items[0].IsGroup = true
	result := Evaluate(ev)
	if result.Sufficient || len(result.Findings) != 0 {
		t.Fatalf("nested visual was treated as supported: %+v", result)
	}
}

func TestUnknownValuesAndMissingSourceNeverHealthy(t *testing.T) {
	now := time.Now()
	ev := baseEvidence(now)
	ev.Observation.Mute = nil
	ev.Observation.VisualItemEnabled = nil
	result := Evaluate(ev)
	if result.Sufficient || len(result.Findings) != 0 {
		t.Fatalf("nil evidence became a result: %+v", result)
	}
	ev = baseEvidence(now)
	ev.Discovery.Scenes = nil
	result = Evaluate(ev)
	if result.Sufficient || len(result.Findings) != 0 {
		t.Fatalf("missing visual source became a finding: %+v", result)
	}
	ev = baseEvidence(now)
	ev.Observation.ObservedAt = now.Add(2 * time.Second)
	result = Evaluate(ev)
	if result.Sufficient {
		t.Fatalf("future-invalid timestamp was accepted: %+v", result)
	}
}
