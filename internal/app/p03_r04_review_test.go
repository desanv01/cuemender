//go:build reviewrepro

package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/incidents"
	"github.com/desanv01/cuemender/internal/twitch"
)

// This is a local composition boundary test, not the protocol/live acceptance path.
// The fixed clock ensures timestamp comparisons cannot substitute for session identity.
func reviewR04QueueApp(t *testing.T) (*App, time.Time) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	now := time.Now().UTC()
	a, err := newWithDependencies(context.Background(), Options{
		DataDir: t.TempDir(), ListenAddress: fmt.Sprintf("127.0.0.1:%d", port),
		NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard,
		Now: func() time.Time { return now },
	}, appDependencies{monitoringStatus: func() incidents.Readiness {
		return incidents.Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, ResolutionReady: true}
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	// Stop only this isolated fixture's consumer so we can deterministically advance
	// monitoring between capture and processing; both production methods are used.
	a.intakeCancel()
	a.intakeWG.Wait()
	a.intakeCtx, a.intakeCancel = context.WithCancel(context.Background())
	track := 1
	_, err = a.SaveSettings(context.Background(), 0, domain.Expectations{
		Microphone: &domain.InputRef{Name: "Review Mic", UUID: "review-mic", Context: "Review"}, StreamTrack: &track,
	})
	if err != nil {
		t.Fatal(err)
	}
	return a, now
}

func TestReviewR04IdleQueuedReportCannotEnterLaterSession(t *testing.T) {
	a, now := reviewR04QueueApp(t)
	a.onTwitchCommand(twitch.CommandEvent{ActorID: "review-viewer", DeliveryID: "review-idle", MessageID: "review-idle-msg",
		ReceivedAt: now, ConnectionGeneration: 1, Command: twitch.Command{Kind: twitch.CommandIssueAudio, Category: "audio"}})
	event := <-a.intakeCh
	if event.MonitoringSessionID != "" {
		t.Fatal("precondition: idle event acquired a session")
	}
	if _, err := a.StartMonitoring(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	a.processTwitchCommand(event)
	list, err := a.store.ListIncidents(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("idle queued report was adopted by later session: %d durable incidents", len(list))
	}
}

func TestReviewR04QueuedOldSessionReportIsDropped(t *testing.T) {
	a, now := reviewR04QueueApp(t)
	first, err := a.StartMonitoring(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	a.onTwitchCommand(twitch.CommandEvent{ActorID: "review-viewer", DeliveryID: "review-old", MessageID: "review-old-msg",
		ReceivedAt: now, ConnectionGeneration: 1, Command: twitch.Command{Kind: twitch.CommandIssueAudio, Category: "audio"}})
	event := <-a.intakeCh
	if event.MonitoringSessionID != first.ID {
		t.Fatal("precondition: origin session was not captured")
	}
	if err := a.StopMonitoring(context.Background()); err != nil {
		t.Fatal(err)
	}
	second, err := a.StartMonitoring(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID {
		t.Fatal("precondition: session was not replaced")
	}
	a.processTwitchCommand(event)
	list, err := a.store.ListIncidents(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("old session event created %d incidents in later session", len(list))
	}
}
