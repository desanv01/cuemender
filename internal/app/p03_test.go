package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/incidents"
	"github.com/desanv01/cuemender/internal/obs"
	"github.com/desanv01/cuemender/internal/store"
	"github.com/desanv01/cuemender/internal/twitch"
)

type p03Backend struct{}

func (p03Backend) Version(context.Context) (obs.VersionInfo, error) {
	return obs.VersionInfo{OBSVersion: "32.1.1", ObsWebSocketVersion: "5.7.3", RPCVersion: 1}, nil
}
func (p03Backend) Inputs(context.Context) ([]domain.OBSInput, error) {
	return []domain.OBSInput{{Name: "Test Mic", UUID: "mic-uuid"}}, nil
}
func (p03Backend) InputMute(context.Context, domain.InputRef) (*bool, error) {
	value := true
	return &value, nil
}
func (p03Backend) InputAudioTracks(context.Context, domain.InputRef) (map[string]bool, error) {
	return map[string]bool{"track1": false, "track2": false, "track3": false, "track4": false, "track5": false, "track6": false}, nil
}
func (p03Backend) ProgramScene(context.Context) (string, string, error) {
	return "Main", "scene-uuid", nil
}
func (p03Backend) SceneCollection(context.Context) (string, error) { return "Collection", nil }
func (p03Backend) Scenes(context.Context) ([]domain.OBSScene, error) {
	return []domain.OBSScene{{Name: "Main", UUID: "scene-uuid"}}, nil
}
func (p03Backend) SceneItems(context.Context, string) ([]domain.OBSSceneItem, error) {
	return nil, nil
}
func (p03Backend) SceneItemEnabled(context.Context, string, int) (*bool, error) {
	value := true
	return &value, nil
}
func (p03Backend) StreamStatus(context.Context) (*bool, error) {
	value := false
	return &value, nil
}
func (p03Backend) Close() error { return nil }

type p03Dialer struct{ backend p03Backend }

func (d p03Dialer) Connect(context.Context, string, string) (obs.Backend, error) {
	return d.backend, nil
}

type blockingBackend struct {
	p03Backend
	entered chan struct{}
	block   atomic.Bool
}

func (b *blockingBackend) InputMute(ctx context.Context, ref domain.InputRef) (*bool, error) {
	if !b.block.Load() {
		return b.p03Backend.InputMute(ctx, ref)
	}
	close(b.entered)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestP03SaveSettingsInvalidatesBlockedDiagnosticAndRecheck(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	backend := &blockingBackend{entered: make(chan struct{})}
	a, err := newWithDependencies(context.Background(), Options{
		DataDir: t.TempDir(), ListenAddress: fmt.Sprintf("127.0.0.1:%d", port),
		NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard,
	}, appDependencies{
		configureOBS: func(opts *obs.Options) { opts.Dialer = blockingDialer{backend: backend} },
		monitoringStatus: func() incidents.Readiness {
			return incidents.Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, ResolutionReady: true}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ctx := context.Background()
	track := 2
	settings, err := a.SaveSettings(ctx, 0, domain.Expectations{Microphone: &domain.InputRef{Name: "Test Mic", UUID: "mic-uuid", Context: "Collection"}, StreamTrack: &track})
	if err != nil {
		t.Fatal(err)
	}
	password := ""
	if err := a.OBSConnect(ctx, "ws://127.0.0.1:4455", &password); err != nil {
		t.Fatal(err)
	}
	waitP03Obs(t, a)
	backend.block.Store(true)
	if _, err := a.StartMonitoring(ctx, settings.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CheckIncident(ctx, "audio", settings.Revision, ""); err != nil {
		t.Fatal(err)
	}
	select {
	case <-backend.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("diagnostic did not block")
	}
	updated, err := a.SaveSettings(ctx, settings.Revision, domain.Expectations{Microphone: &domain.InputRef{Name: "Test Mic", UUID: "mic-uuid", Context: "Collection"}, StreamTrack: &track})
	if err != nil {
		t.Fatal(err)
	}
	_, list := a.coordinator.Snapshot()
	if len(list) == 0 || list[0].Stage != domain.StageInterrupted || len(list[0].Findings) != 0 {
		t.Fatalf("stale blocked result was not invalidated: %+v", list)
	}
	backend.block.Store(false)
	if _, err := a.CheckIncident(ctx, "audio", updated.Revision, list[0].ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := a.coordinator.Find(list[0].ID)
		if len(got.Findings) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("explicit recheck after settings change did not diagnose")
}

func TestP03BlockedDiagnosticWholeAppShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	dataDir := t.TempDir()
	backend := &blockingBackend{entered: make(chan struct{})}
	a, err := newWithDependencies(context.Background(), Options{
		DataDir: dataDir, ListenAddress: fmt.Sprintf("127.0.0.1:%d", port),
		NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard,
	}, appDependencies{
		configureOBS: func(opts *obs.Options) { opts.Dialer = blockingDialer{backend: backend} },
		monitoringStatus: func() incidents.Readiness {
			return incidents.Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, ResolutionReady: true}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan error, 1)
	go func() { runDone <- a.Run(ctx) }()
	waitHealthy(t, a.BaseURL())
	sseResp, err := http.Get(a.BaseURL() + "/public/v1/events")
	if err != nil {
		t.Fatal(err)
	}
	sseInitial := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(sseResp.Body)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "data: ") {
				close(sseInitial)
				break
			}
		}
	}()
	select {
	case <-sseInitial:
	case <-time.After(2 * time.Second):
		t.Fatal("no active SSE snapshot")
	}
	track := 2
	if _, err := a.SaveSettings(ctx, 0, domain.Expectations{Microphone: &domain.InputRef{Name: "Test Mic", UUID: "mic-uuid", Context: "Collection"}, StreamTrack: &track}); err != nil {
		t.Fatal(err)
	}
	password := ""
	if err := a.OBSConnect(ctx, "ws://127.0.0.1:4455", &password); err != nil {
		t.Fatal(err)
	}
	waitP03Obs(t, a)
	backend.block.Store(true)
	if _, err := a.StartMonitoring(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CheckIncident(ctx, "audio", 1, ""); err != nil {
		t.Fatal(err)
	}
	select {
	case <-backend.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("diagnostic did not reach blocked observation")
	}
	closed := make(chan struct{})
	cancel()
	go func() { <-runDone; close(closed) }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("whole-app shutdown blocked on diagnostic")
	}
	_ = sseResp.Body.Close()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err = net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listener not released: %v", err)
	}
	_ = ln.Close()
	db, err := store.Open(context.Background(), filepath.Join(dataDir, databaseFileName))
	if err != nil {
		t.Fatalf("database not reopenable: %v", err)
	}
	_ = db.Close()
}

type blockingDialer struct{ backend obs.Backend }

func (d blockingDialer) Connect(context.Context, string, string) (obs.Backend, error) {
	return d.backend, nil
}

func waitP03Obs(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if a.obsMgr.Snapshot().Status == domain.ConnectionConnected {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("fake OBS did not connect: %+v", a.obsMgr.Snapshot())
}

func TestP03ApplicationWiringPersistsIncidentAndRestart(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	dataDir := t.TempDir()
	a, err := newWithDependencies(context.Background(), Options{
		DataDir: dataDir, ListenAddress: fmt.Sprintf("127.0.0.1:%d", port),
		NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard,
	}, appDependencies{
		configureOBS: func(opts *obs.Options) { opts.Dialer = p03Dialer{backend: p03Backend{}} },
		monitoringStatus: func() incidents.Readiness {
			return incidents.Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, ResolutionReady: true}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	track := 2
	settings, err := a.SaveSettings(ctx, 0, domain.Expectations{
		Microphone: &domain.InputRef{Name: "Test Mic", UUID: "mic-uuid", Context: "Collection"}, StreamTrack: &track,
	})
	if err != nil {
		t.Fatal(err)
	}
	password := ""
	if err := a.OBSConnect(ctx, "ws://127.0.0.1:4455", &password); err != nil {
		t.Fatal(err)
	}
	waitP03Obs(t, a)
	if _, err := a.StartMonitoring(ctx, settings.Revision); err != nil {
		t.Fatal(err)
	}
	a.onTwitchCommand(twitch.CommandEvent{
		ActorID: "viewer-1", DeliveryID: "delivery-1", MessageID: "message-1",
		ConnectionGeneration: 1, ReceivedAt: time.Now().UTC(),
		Command: twitch.Command{Kind: twitch.CommandIssueAudio, Category: "audio"},
	})
	deadline := time.Now().Add(3 * time.Second)
	var found *domain.Incident
	for time.Now().Before(deadline) {
		_, list := a.coordinator.Snapshot()
		for i := range list {
			if list[i].Category == "audio" && len(list[i].Findings) > 0 {
				found = &list[i]
				break
			}
		}
		if found != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if found == nil {
		t.Fatal("report did not persist a finding through application wiring")
	}
	if len(found.Findings) < 2 {
		t.Fatalf("expected mute + track findings, got %+v", found.Findings)
	}
	private, err := a.PrivateSnapshot(ctx)
	if err != nil || !private.MonitoringActive || len(private.Incidents) == 0 {
		t.Fatalf("private projection = %+v, %v", private, err)
	}
	public, err := a.PublicSnapshot(ctx)
	if err != nil || !public.MonitoringActive || len(public.Incidents) != 1 {
		t.Fatalf("public projection = %+v, %v", public, err)
	}
	publicJSON, _ := json.Marshal(public)
	for _, forbidden := range []string{"Test Mic", "mic-uuid", "MIC_MUTED", "findings"} {
		if strings.Contains(string(publicJSON), forbidden) {
			t.Fatalf("public projection leaked %q: %s", forbidden, publicJSON)
		}
	}
	if err := a.StopMonitoring(ctx); err != nil {
		t.Fatal(err)
	}
	public, _ = a.PublicSnapshot(ctx)
	if len(public.Incidents) != 0 {
		t.Fatal("stopped monitoring still exposes public incidents")
	}
	a.Close()
	reopened, err := store.Open(ctx, filepath.Join(dataDir, databaseFileName))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.LoadSettings(ctx)
	if err != nil || loaded.Revision != settings.Revision {
		t.Fatalf("settings after restart = %+v, %v", loaded, err)
	}
	list, err := reopened.ListIncidents(ctx, 10)
	if err != nil || len(list) == 0 {
		t.Fatalf("incident history after restart = %+v, %v", list, err)
	}
}
