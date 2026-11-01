package diagnostics

import (
	"fmt"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

const MaxEvidenceAge = 5 * time.Second

type Evidence struct {
	Settings    domain.Settings
	Discovery   *domain.OBSDiscovery
	Observation *domain.OBSObservation
	Now         time.Time
}

type Result struct {
	Checks     []domain.DiagnosticCheck
	Findings   []domain.Finding
	Sufficient bool
}

// Evaluate performs only the three supported read-only comparisons. It never
// converts an unavailable value into a healthy value.
func Evaluate(ev Evidence) Result {
	result := Result{}
	if ev.Observation == nil {
		for _, category := range []string{"audio", "video"} {
			result.Checks = append(result.Checks, domain.DiagnosticCheck{
				Name: category + "_evidence", Category: category, Status: "unknown",
				Explanation: "observation_unavailable", Unavailable: "OBS observation is unavailable.",
			})
		}
		return result
	}
	stale := ev.Now.Sub(ev.Observation.ObservedAt) > MaxEvidenceAge ||
		ev.Observation.ObservedAt.After(ev.Now.Add(time.Second))
	if ev.Settings.Revision != ev.Observation.ConfigRevision || ev.Discovery == nil ||
		ev.Discovery.ConfigRevision != ev.Settings.Revision ||
		ev.Discovery.Generation != ev.Observation.Generation ||
		(ev.Discovery.SceneCollection != "" && ev.Observation.SceneCollection != ev.Discovery.SceneCollection) {
		stale = true
	}

	micCheck := evaluateMute(ev, stale)
	result.Checks = append(result.Checks, micCheck)
	trackCheck := evaluateTrack(ev, stale)
	result.Checks = append(result.Checks, trackCheck)
	visualCheck := evaluateVisual(ev, stale)
	result.Checks = append(result.Checks, visualCheck)

	for _, check := range result.Checks {
		if check.Status == "conflict" {
			result.Findings = append(result.Findings, findingFor(check, ev))
		}
	}
	result.Sufficient = allRelevantSufficient(result.Checks)
	return result
}

func evaluateMute(ev Evidence, stale bool) domain.DiagnosticCheck {
	check := domain.DiagnosticCheck{Name: "microphone_muted", Category: "audio"}
	if ev.Settings.Microphone == nil {
		check.Status, check.Explanation = "not_configured", "microphone_not_configured"
		return check
	}
	if stale {
		check.Status, check.Explanation = "unknown", "evidence_stale"
		return check
	}
	if ev.Observation.Mute == nil {
		check.Status, check.Explanation = "unknown", "mute_unknown"
		return check
	}
	check.Observed = map[string]any{"mute": *ev.Observation.Mute}
	if *ev.Observation.Mute {
		check.Status, check.Explanation = "conflict", "microphone_muted"
		return check
	}
	check.Status, check.Explanation = "ok", "microphone_unmuted"
	return check
}

func evaluateTrack(ev Evidence, stale bool) domain.DiagnosticCheck {
	check := domain.DiagnosticCheck{Name: "microphone_track", Category: "audio"}
	if ev.Settings.Microphone == nil || ev.Settings.StreamTrack == nil {
		check.Status, check.Explanation = "not_configured", "stream_track_not_configured"
		return check
	}
	if stale {
		check.Status, check.Explanation = "unknown", "evidence_stale"
		return check
	}
	if !CompleteTracks(ev.Observation.Tracks) {
		check.Status, check.Explanation = "unknown", "tracks_incomplete"
		return check
	}
	key := fmt.Sprintf("track%d", *ev.Settings.StreamTrack)
	enabled, ok := ev.Observation.Tracks[key]
	if !ok {
		check.Status, check.Explanation = "unknown", "track_unknown"
		return check
	}
	check.Observed = map[string]any{"track": *ev.Settings.StreamTrack, "enabled": enabled}
	if !enabled {
		check.Status, check.Explanation = "conflict", "microphone_missing_from_track"
		return check
	}
	check.Status, check.Explanation = "ok", "microphone_present_on_track"
	return check
}

func evaluateVisual(ev Evidence, stale bool) domain.DiagnosticCheck {
	check := domain.DiagnosticCheck{Name: "visual_item", Category: "video"}
	visual := ev.Settings.Visual
	if visual == nil {
		check.Status, check.Explanation = "not_configured", "visual_not_configured"
		return check
	}
	if stale {
		check.Status, check.Explanation = "unknown", "evidence_stale"
		return check
	}
	if ev.Observation.ProgramScene == "" {
		check.Status, check.Explanation = "unknown", "program_scene_unknown"
		return check
	}
	if visual.SceneUUID != "" && ev.Observation.ProgramSceneUUID != "" && visual.SceneUUID != ev.Observation.ProgramSceneUUID {
		check.Status, check.Explanation = "ok", "configured_item_not_in_program_scene"
		return check
	}
	if visual.Scene != "" && visual.Scene != ev.Observation.ProgramScene {
		check.Status, check.Explanation = "ok", "configured_item_not_in_program_scene"
		return check
	}
	item, _ := findItem(ev.Discovery, *visual)
	if item == nil {
		check.Status, check.Explanation = "unknown", "visual_identity_unresolved"
		return check
	}
	if item.IsGroup {
		check.Status, check.Explanation = "unknown", "nested_visual_unsupported"
		return check
	}
	if ev.Observation.VisualItemEnabled == nil {
		check.Status, check.Explanation = "unknown", "visual_visibility_unknown"
		return check
	}
	check.Observed = map[string]any{"enabled": *ev.Observation.VisualItemEnabled}
	if !*ev.Observation.VisualItemEnabled {
		check.Status, check.Explanation = "conflict", "visual_item_hidden"
		return check
	}
	check.Status, check.Explanation = "ok", "visual_item_visible"
	return check
}

func findItem(discovery *domain.OBSDiscovery, visual domain.VisualExpectation) (*domain.OBSSceneItem, *domain.OBSScene) {
	if discovery == nil {
		return nil, nil
	}
	for i := range discovery.Scenes {
		scene := &discovery.Scenes[i]
		if visual.Scene != "" && scene.Name != visual.Scene {
			continue
		}
		if visual.SceneUUID != "" && scene.UUID != visual.SceneUUID {
			continue
		}
		for j := range scene.Items {
			item := &scene.Items[j]
			if visual.SceneItemID != "" && fmt.Sprint(item.ID) != visual.SceneItemID {
				continue
			}
			if visual.Source.Name != "" && item.SourceName != visual.Source.Name {
				continue
			}
			if visual.Source.UUID != "" && item.SourceUUID != visual.Source.UUID {
				continue
			}
			return item, scene
		}
	}
	return nil, nil
}

func findingFor(check domain.DiagnosticCheck, ev Evidence) domain.Finding {
	finding := domain.Finding{
		Code:                check.Explanation,
		Category:            check.Category,
		EvidenceStatus:      domain.EvidenceKnown,
		ExplanationCode:     check.Explanation,
		ExpectationRevision: ev.Settings.Revision,
		ObservedAt:          ev.Observation.ObservedAt,
		Observed:            check.Observed,
		ManualGuidance:      guidance(check.Explanation),
	}
	finding.ConnectionGeneration = ev.Observation.Generation
	finding.Context = ev.Observation.SceneCollection
	switch check.Explanation {
	case "microphone_muted":
		finding.Code = domain.FindingMICMuted
		finding.ManualGuidance = "The microphone is muted in OBS. Unmute it manually; CueMender does not perform this action."
	case "microphone_missing_from_track":
		finding.Code = domain.FindingMICTrackMissing
		finding.SupportedActionKind = domain.ActionAddMicToTrack
		finding.ManualGuidance = "The microphone is absent from the confirmed stream track. Repair is not available until the approved repair phase."
	case "visual_item_hidden":
		finding.Code = domain.FindingVisualHidden
		finding.ManualGuidance = "The configured scene item is hidden. Reveal it manually in OBS; CueMender does not change visibility."
	}
	return finding
}

func guidance(explanation string) string {
	switch explanation {
	case "microphone_muted":
		return "Check the microphone mute state in OBS and unmute it manually."
	case "microphone_missing_from_track":
		return "Check the microphone's audio track assignment in OBS."
	case "visual_item_hidden":
		return "Check the configured scene item's visibility in OBS."
	default:
		return "Inspect OBS manually; CueMender has no supported automatic action."
	}
}

func CompleteTracks(tracks map[string]bool) bool {
	if len(tracks) != 6 {
		return false
	}
	for i := 1; i <= 6; i++ {
		if _, ok := tracks[fmt.Sprintf("track%d", i)]; !ok {
			return false
		}
	}
	return true
}

func allRelevantSufficient(checks []domain.DiagnosticCheck) bool {
	for _, check := range checks {
		if check.Status == "unknown" {
			return false
		}
	}
	return true
}
