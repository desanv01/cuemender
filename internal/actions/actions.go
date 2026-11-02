// Package actions defines the narrow proposal and authority checks used by the
// incident coordinator. It has no transport, HTTP or persistence authority.
package actions

import (
	"fmt"
	"reflect"
	"time"

	"github.com/desanv01/cuemender/internal/diagnostics"
	"github.com/desanv01/cuemender/internal/domain"
)

const ProposalTTL = 60 * time.Second

func CloneTracks(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func Proposal(id string, inc domain.Incident, settings domain.Settings, discovery *domain.OBSDiscovery, observation *domain.OBSObservation, now time.Time) (domain.Proposal, bool) {
	if settings.Microphone == nil || settings.StreamTrack == nil || settings.TrackConfirmedAt == nil || *settings.StreamTrack < 1 || *settings.StreamTrack > 6 || discovery == nil || !discovery.CollectionKnown || discovery.SceneCollection == "" || observation == nil || (len(observation.Tracks) != 6 || !diagnostics.CompleteTracks(observation.Tracks)) || now.Sub(observation.ObservedAt) > diagnostics.MaxEvidenceAge || observation.ObservedAt.After(now.Add(time.Second)) {
		return domain.Proposal{}, false
	}
	if settings.Revision != observation.ConfigRevision || observation.Generation != discovery.Generation || observation.SceneCollection != discovery.SceneCollection {
		return domain.Proposal{}, false
	}
	input := *settings.Microphone
	original := input
	input.Context = discovery.SceneCollection
	matches := 0
	for _, candidate := range discovery.Inputs {
		if candidate.Name == input.Name && (original.UUID == "" || candidate.UUID == original.UUID) {
			input.UUID = candidate.UUID
			matches++
		}
	}
	if observation.Microphone == nil || observation.Microphone.Name != input.Name || (original.UUID != "" && observation.Microphone.UUID != original.UUID) || (original.Context != "" && original.Context != input.Context) {
		return domain.Proposal{}, false
	}
	if matches != 1 {
		return domain.Proposal{}, false
	}
	key := fmt.Sprintf("track%d", *settings.StreamTrack)
	if observation.Tracks[key] {
		return domain.Proposal{}, false
	}
	desired := CloneTracks(observation.Tracks)
	desired[key] = true
	return domain.Proposal{ID: id, IncidentID: inc.ID, SessionID: inc.SessionID, Mode: inc.Mode, Kind: domain.ActionAddMicToTrack, Input: input, ExpectedTracks: CloneTracks(observation.Tracks), DesiredTracks: desired, SelectedTrack: *settings.StreamTrack, ConfigRevision: settings.Revision, Generation: observation.Generation, CreatedAt: now.UTC(), ExpiresAt: now.Add(ProposalTTL).UTC(), Status: domain.ProposalProposed}, true
}

func Validate(p domain.Proposal, session *domain.MonitoringSessionInfo, settings domain.Settings, generation uint64, connected bool, now time.Time) error {
	if !now.Before(p.ExpiresAt) {
		return domain.ErrProposalExpired
	}
	if !connected || generation != p.Generation || settings.Revision != p.ConfigRevision || p.Mode != domain.ModeLive || session == nil || !session.Active || session.ID != p.SessionID || settings.Microphone == nil || settings.StreamTrack == nil || settings.TrackConfirmedAt == nil || *settings.StreamTrack != p.SelectedTrack {
		return domain.ErrActionConflict
	}
	mic := *settings.Microphone
	if mic.Name != p.Input.Name || (mic.UUID != "" && mic.UUID != p.Input.UUID) || (mic.Context != "" && mic.Context != p.Input.Context) {
		return domain.ErrActionConflict
	}
	return nil
}

func MatchesPreflight(p domain.Proposal, f domain.TrackRepairPreflight) bool {
	return f.State.Generation == p.Generation && f.State.ConfigRevision == p.ConfigRevision && f.State.Input == p.Input && f.SelectedTrack == p.SelectedTrack && reflect.DeepEqual(f.State.Tracks, p.ExpectedTracks) && reflect.DeepEqual(f.DesiredTracks, p.DesiredTracks)
}
