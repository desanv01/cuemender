package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
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

func TestP05RehearsalAllScenarios(t *testing.T) {
	for _, scenario := range []string{"track_missing", "mic_muted", "wrong_scene", "unknown_write"} {
		t.Run(scenario, func(t *testing.T) {
			a := p05ReviewApp(t)
			ctx := context.Background()
			settings, err := a.RehearsalScenario(ctx, scenario)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.StartMonitoring(ctx, settings.Revision); err != nil {
				t.Fatal(err)
			}
			a.RehearsalFeedback(ctx, "unknown", "fixed")
			category := "audio"
			if scenario == "wrong_scene" {
				category = "video"
			}
			inc, err := a.RehearsalReport(ctx, category)
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				got, _ := a.coordinator.Find(inc.ID)
				if got.Stage != domain.StageReported && got.Stage != domain.StageInvestigating {
					inc = got
					break
				}
				time.Sleep(time.Millisecond)
			}
			if inc.Category != category {
				t.Fatalf("report category=%s want %s", inc.Category, category)
			}
			switch scenario {
			case "track_missing":
				if inc.Stage != domain.StageAwaitingApproval || len(inc.Findings) == 0 {
					t.Fatalf("track_missing stage=%s findings=%d", inc.Stage, len(inc.Findings))
				}
				proposals, _ := a.coordinator.RepairSnapshot()
				if len(proposals) == 0 {
					t.Fatal("no proposal")
				}
				if _, err := a.ApproveProposal(ctx, proposals[0].ID, proposals[0].ConfigRevision); err != nil {
					t.Fatal(err)
				}
				wait := time.Now().Add(3 * time.Second)
				for time.Now().Before(wait) {
					_, actions := a.coordinator.RepairSnapshot()
					if len(actions) > 0 && actions[0].Status == domain.TrackRepairVerified {
						break
					}
					time.Sleep(time.Millisecond)
				}
			case "mic_muted":
				if inc.Stage != domain.StageAwaitingFeedback {
					t.Fatalf("mic_muted stage=%s", inc.Stage)
				}
			case "wrong_scene":
				if inc.Stage != domain.StageInsufficientInfo {
					t.Fatalf("wrong_scene stage=%s", inc.Stage)
				}
			case "unknown_write":
				if inc.Stage != domain.StageAwaitingApproval {
					t.Fatalf("unknown_write stage=%s", inc.Stage)
				}
				proposals, _ := a.coordinator.RepairSnapshot()
				if len(proposals) == 0 {
					t.Fatal("no proposal")
				}
				if _, err := a.ApproveProposal(ctx, proposals[0].ID, proposals[0].ConfigRevision); err != nil {
					t.Fatal(err)
				}
				wait := time.Now().Add(3 * time.Second)
				for time.Now().Before(wait) {
					_, actions := a.coordinator.RepairSnapshot()
					if len(actions) > 0 && actions[0].Status == domain.TrackRepairUnknown {
						return
					}
					time.Sleep(time.Millisecond)
				}
				t.Fatal("unknown_write did not settle unknown")
			}
		})
	}
}

func TestP05RehearsalPrivateHTTPFlow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := New(ctx, Options{Mode: domain.ModeRehearsal, DataDir: t.TempDir(), ListenAddress: freeP05Address(t), NoBrowser: true, Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	waitHealthy(t, a.BaseURL())
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	post := func(path string, body any, csrf string, want int) []byte {
		t.Helper()
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", a.BaseURL()+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", a.BaseURL())
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		out, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s=%d want %d: %s", path, res.StatusCode, want, out)
		}
		return out
	}
	var boot struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		}
	}
	_ = json.Unmarshal(post("/api/v1/session/bootstrap", map[string]any{"token": a.ui.BootstrapToken()}, "", 200), &boot)
	csrf := boot.Data.CSRF
	var scenario struct{ Data domain.Settings }
	_ = json.Unmarshal(post("/api/v1/rehearsal/scenario", map[string]any{"scenario": "mic_muted"}, csrf, 200), &scenario)
	post("/api/v1/monitoring/start", map[string]any{"expected_revision": scenario.Data.Revision}, csrf, 200)
	post("/api/v1/rehearsal/report", map[string]any{"category": "audio"}, csrf, 202)
	var incident domain.Incident
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, list := a.coordinator.Snapshot()
		if len(list) > 0 && list[0].Stage == domain.StageAwaitingFeedback {
			incident = list[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if incident.ID == "" {
		t.Fatal("rehearsal HTTP report did not settle awaiting_feedback")
	}
	post("/api/v1/rehearsal/feedback", map[string]any{"public_id": incident.PublicID, "vote": "fixed"}, csrf, 202)
	var voted struct{ Data domain.Incident }
	_ = json.Unmarshal(post("/api/v1/rehearsal/feedback", map[string]any{"public_id": incident.PublicID, "vote": "stillbroken"}, csrf, 202), &voted)
	if voted.Data.Feedback.Positive != 0 || voted.Data.Feedback.Negative != 1 {
		t.Fatalf("HTTP feedback replacement=%+v", voted.Data.Feedback)
	}
	post("/api/v1/incidents/"+incident.ID+"/close", map[string]any{"outcome": "unresolved"}, csrf, 200)
	post("/api/v1/rehearsal/feedback", map[string]any{"public_id": incident.PublicID, "vote": "fixed"}, csrf, 409)
	var wrongScene struct{ Data domain.Settings }
	_ = json.Unmarshal(post("/api/v1/rehearsal/scenario", map[string]any{"scenario": "wrong_scene"}, csrf, 200), &wrongScene)
	post("/api/v1/monitoring/start", map[string]any{"expected_revision": wrongScene.Data.Revision}, csrf, 200)
	post("/api/v1/rehearsal/report", map[string]any{"category": "video"}, csrf, 202)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("rehearsal HTTP app did not stop")
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
