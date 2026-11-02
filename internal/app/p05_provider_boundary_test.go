package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/desanv01/cuemender/internal/domain"
)

func TestP05ProviderRejectsCanceledAndStaleReads(t *testing.T) {
	r := newRehearsalState(domain.Settings{Revision: 3})
	input := domain.InputRef{Name: "Rehearsal Mic", UUID: "rehearsal-mic", Context: "Rehearsal"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := r.Read(ctx, domain.Settings{Revision: 3}); !errors.Is(err, context.Canceled) {
		t.Fatalf("read cancellation: %v", err)
	}
	if _, err := r.ReadTrack(ctx, domain.TrackStateRequest{Input: input, Generation: 1, ConfigRevision: 3}); !errors.Is(err, context.Canceled) {
		t.Fatalf("readback cancellation: %v", err)
	}
	if _, _, err := r.Read(context.Background(), domain.Settings{Revision: 2}); err == nil {
		t.Fatal("stale read relabelled provider")
	}
	got, err := r.ReadTrack(context.Background(), domain.TrackStateRequest{Input: input, Generation: 1, ConfigRevision: 3})
	if err != nil || got.ConfigRevision != 3 {
		t.Fatalf("stale read corrupted current provenance: %+v %v", got, err)
	}
}

func TestP05ProviderRejectsMissingFalseKey(t *testing.T) {
	r := newRehearsalState(domain.Settings{Revision: 1})
	expected := cloneTrackMap(r.tracks)
	delete(expected, "track2")
	expected["bogus"] = false
	called := false
	got := r.Repair(context.Background(), domain.TrackRepairRequest{Input: domain.InputRef{Name: "Rehearsal Mic", UUID: "rehearsal-mic", Context: "Rehearsal"}, Generation: 1, ConfigRevision: 1, ExpectedTracks: expected, SelectedTrack: 2}, func(domain.TrackRepairPreflight) error { called = true; return nil })
	if called || r.written || got.Status != domain.TrackRepairInvalidated {
		t.Fatalf("malformed map admitted: %+v", got)
	}
}

func TestP05ProviderAlreadyDesiredIsNoOp(t *testing.T) {
	r := newRehearsalState(domain.Settings{Revision: 1})
	expected := cloneTrackMap(r.tracks)
	r.tracks["track2"] = true
	called := false
	got := r.Repair(context.Background(), domain.TrackRepairRequest{Input: domain.InputRef{Name: "Rehearsal Mic", UUID: "rehearsal-mic", Context: "Rehearsal"}, Generation: 1, ConfigRevision: 1, ExpectedTracks: expected, SelectedTrack: 2}, func(domain.TrackRepairPreflight) error { called = true; return nil })
	if called || r.written || got.Status != domain.TrackRepairNoOp || got.Observed == nil {
		t.Fatalf("already desired configuration dispatched: %+v", got)
	}
}

func TestP05ProviderRevalidatesAfterAdmission(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		t.Run(map[bool]string{false: "canceled", true: "replaced"}[replacement], func(t *testing.T) {
			r := newRehearsalState(domain.Settings{Revision: 1})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req := domain.TrackRepairRequest{Input: domain.InputRef{Name: "Rehearsal Mic", UUID: "rehearsal-mic", Context: "Rehearsal"}, Generation: 1, ConfigRevision: 1, ExpectedTracks: cloneTrackMap(r.tracks), SelectedTrack: 2}
			got := r.Repair(ctx, req, func(domain.TrackRepairPreflight) error {
				if replacement {
					r.setScenario("mic_muted")
				} else {
					cancel()
				}
				return nil
			})
			if got.Status != domain.TrackRepairInvalidated || r.written {
				t.Fatalf("invalidated operation wrote: %+v", got)
			}
			if replacement && !reflect.DeepEqual(r.tracks, trackMap(true)) {
				t.Fatal("replacement map changed")
			}
		})
	}
}
