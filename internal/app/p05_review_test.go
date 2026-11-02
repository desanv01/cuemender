// Promoted from Codex's P05-R01 review set; assertions preserved.
package app

import (
	"context"
	"github.com/desanv01/cuemender/internal/domain"
	"io"
	"testing"
	"time"
)

func p05ReviewApp(t *testing.T) *App {
	t.Helper()
	a, err := New(context.Background(), Options{Mode: domain.ModeRehearsal, DataDir: t.TempDir(), ListenAddress: freeP05Address(t), NoBrowser: true, Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

func TestReviewP05RehearsalSelectedTrackAndReadback(t *testing.T) {
	r := newRehearsalState(domain.Settings{Revision: 1})
	r.tracks["track2"] = true
	r.tracks["track3"] = false
	req := domain.TrackRepairRequest{ActionID: "a", Input: domain.InputRef{Name: "Rehearsal Mic", UUID: "rehearsal-mic", Context: "Rehearsal"}, Generation: 1, ConfigRevision: 1, ExpectedTracks: cloneTrackMap(r.tracks), SelectedTrack: 3}
	got := r.Repair(context.Background(), req, func(domain.TrackRepairPreflight) error { return nil })
	observed, err := r.ReadTrack(context.Background(), domain.TrackStateRequest{Input: req.Input, Generation: 1, ConfigRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !observed.Tracks["track3"] || got.Status != domain.TrackRepairVerified || got.Observed == nil {
		t.Fatalf("verified without requested change/readback: tracks=%v result=%+v", observed.Tracks, got)
	}
}

func TestReviewP05RehearsalPreflightUsesActualMap(t *testing.T) {
	r := newRehearsalState(domain.Settings{Revision: 1})
	expected := cloneTrackMap(r.tracks)
	r.tracks["track1"] = false
	calls := 0
	got := r.Repair(context.Background(), domain.TrackRepairRequest{ActionID: "a", Input: domain.InputRef{Name: "Rehearsal Mic", UUID: "rehearsal-mic", Context: "Rehearsal"}, Generation: 1, ConfigRevision: 1, ExpectedTracks: expected, SelectedTrack: 2}, func(domain.TrackRepairPreflight) error { calls++; return nil })
	if calls != 0 || got.Status != domain.TrackRepairInvalidated || r.written {
		t.Fatalf("changed real fixture map admitted: callbacks=%d result=%+v", calls, got)
	}
}

func TestReviewP05RehearsalCanceledRepairCannotWrite(t *testing.T) {
	r := newRehearsalState(domain.Settings{Revision: 1})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	got := r.Repair(ctx, domain.TrackRepairRequest{ActionID: "a", ExpectedTracks: cloneTrackMap(r.tracks), SelectedTrack: 2}, func(domain.TrackRepairPreflight) error { calls++; return nil })
	if calls != 0 || r.written || got.Status == domain.TrackRepairVerified {
		t.Fatalf("canceled simulated write: callbacks=%d written=%v status=%s", calls, r.written, got.Status)
	}
}

func TestReviewP05FailedScenarioDoesNotMutateProvider(t *testing.T) {
	a := p05ReviewApp(t)
	before := a.rehearsal.scenario
	if err := a.store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RehearsalScenario(context.Background(), "mic_muted"); err == nil {
		t.Fatal("expected settings persistence failure")
	}
	if a.rehearsal.scenario != before {
		t.Fatalf("failed scenario partially installed provider: before=%s after=%s", before, a.rehearsal.scenario)
	}
}

func TestReviewP05ReportResponseMatchesCategory(t *testing.T) {
	a := p05ReviewApp(t)
	ctx := context.Background()
	settings, err := a.RehearsalScenario(ctx, "track_missing")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.StartMonitoring(ctx, settings.Revision); err != nil {
		t.Fatal(err)
	}
	settle := func(inc domain.Incident) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			got, _ := a.coordinator.Find(inc.ID)
			if got.Stage != domain.StageInvestigating && got.Stage != domain.StageReported {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("diagnostic did not settle")
	}
	audio, err := a.RehearsalReport(ctx, "audio")
	if err != nil {
		t.Fatal(err)
	}
	settle(audio)
	video, err := a.RehearsalReport(ctx, "video")
	if err != nil {
		t.Fatal(err)
	}
	if video.Category != "video" {
		t.Fatalf("video report response returned another category: %+v", video)
	}
	settle(video)
	joined, err := a.RehearsalReport(ctx, "audio")
	if err != nil {
		t.Fatal(err)
	}
	if joined.Category != "audio" || joined.ID != audio.ID {
		t.Fatalf("audio response returned another incident: audio=%s result=%+v", audio.ID, joined)
	}
}

func TestReviewP05RehearsalFeedbackRejectsClosedAndRecordsNegative(t *testing.T) {
	a := p05ReviewApp(t)
	ctx := context.Background()
	settings, err := a.RehearsalScenario(ctx, "mic_muted")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.StartMonitoring(ctx, settings.Revision); err != nil {
		t.Fatal(err)
	}
	inc, err := a.RehearsalReport(ctx, "audio")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := a.coordinator.Find(inc.ID)
		if got.Stage == domain.StageAwaitingFeedback {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, err = a.RehearsalFeedback(ctx, inc.PublicID, "fixed"); err != nil {
		t.Fatal(err)
	}
	got, err := a.RehearsalFeedback(ctx, inc.PublicID, "stillbroken")
	if err != nil {
		t.Fatal(err)
	}
	if got.Feedback.Positive != 0 || got.Feedback.Negative != 1 {
		t.Errorf("negative vote did not replace positive: %+v", got.Feedback)
	}
	if _, err = a.CloseIncident(ctx, inc.ID, "unresolved"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.RehearsalFeedback(ctx, inc.PublicID, "fixed"); err == nil {
		t.Error("closed-incident feedback reported success")
	}
}
