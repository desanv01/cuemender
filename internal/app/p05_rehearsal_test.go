package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/obs"
	"github.com/desanv01/cuemender/internal/twitch"
)

func TestP05RehearsalIsolationAndScenario(t *testing.T) {
	var osCalls, twitchCfg, obsCfg atomic.Int32
	base := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	a, err := newWithDependencies(context.Background(), Options{
		Mode: domain.ModeRehearsal, DataDir: base, ListenAddress: fmt.Sprintf("127.0.0.1:%d", port),
		NoBrowser: true, Stdout: io.Discard, Stderr: io.Discard,
	}, appDependencies{
		newOSStore:      func() osSecretStore { osCalls.Add(1); return nil },
		configureTwitch: func(*twitch.Options) { twitchCfg.Add(1) },
		configureOBS:    func(*obs.Options) { obsCfg.Add(1) },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if osCalls.Load() != 0 || twitchCfg.Load() != 0 || obsCfg.Load() != 0 {
		t.Fatalf("rehearsal constructed live dependencies: keyring=%d twitch=%d obs=%d", osCalls.Load(), twitchCfg.Load(), obsCfg.Load())
	}
	if a.Mode() != domain.ModeRehearsal || !strings.HasSuffix(a.DataDir(), "rehearsal") {
		t.Fatalf("mode=%s data=%s", a.Mode(), a.DataDir())
	}
	if a.twitchMgr != nil || a.obsMgr != nil {
		t.Fatal("rehearsal constructed live manager")
	}
	if err := a.TwitchConnect(context.Background(), "abc123"); !errors.Is(err, domain.ErrModeConflict) {
		t.Fatalf("rehearsal TwitchConnect=%v", err)
	}
	password := ""
	if err := a.OBSConnect(context.Background(), "ws://127.0.0.1:4455", &password); !errors.Is(err, domain.ErrModeConflict) {
		t.Fatalf("rehearsal OBSConnect=%v", err)
	}
	ctx := context.Background()
	settings, err := a.RehearsalScenario(ctx, "track_missing")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.StartMonitoring(ctx, settings.Revision); err != nil {
		t.Fatal(err)
	}
	inc, err := a.RehearsalReport(ctx, "audio")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	var proposals []domain.Proposal
	for time.Now().Before(deadline) {
		proposals, _ = a.coordinator.RepairSnapshot()
		if len(proposals) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(proposals) == 0 {
		t.Fatal("rehearsal track_missing did not produce a proposal")
	}
	if _, err := a.ApproveProposal(ctx, proposals[0].ID, proposals[0].ConfigRevision); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, actions := a.coordinator.RepairSnapshot()
		if len(actions) > 0 && actions[0].Status == domain.TrackRepairVerified {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, actions := a.coordinator.RepairSnapshot()
	if len(actions) == 0 || actions[0].Status != domain.TrackRepairVerified {
		t.Fatalf("rehearsal approval did not verify: %+v", actions)
	}
	if inc.Category != "audio" {
		t.Fatalf("incident category=%s", inc.Category)
	}
	got, err := a.CloseIncident(ctx, inc.ID, "unresolved")
	if err != nil || got.Stage != domain.StageClosed || got.ClosureOutcome != "unresolved" {
		t.Fatalf("closure=%+v err=%v", got, err)
	}
	// Cross-mode ID cannot be adopted by a live coordinator.
	live, err := New(context.Background(), Options{DataDir: t.TempDir(), ListenAddress: freeP05Address(t), NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	if _, ok := live.coordinator.Find(inc.ID); ok {
		t.Fatal("rehearsal incident visible to live coordinator")
	}
}

func TestP05InvalidModeRejected(t *testing.T) {
	if a, err := New(context.Background(), Options{Mode: "prod", DataDir: t.TempDir(), NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard}); a != nil || err == nil {
		t.Fatal("unknown mode constructed an app")
	}
}

func freeP05Address(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}
