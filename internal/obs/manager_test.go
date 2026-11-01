package obs

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/secrets"
)

type fakeBackend struct {
	mu         sync.Mutex
	closed     bool
	muteErr    error
	tracksErr  error
	collection string
}

func (b *fakeBackend) Version(context.Context) (VersionInfo, error) {
	return VersionInfo{OBSVersion: "32.1.1", ObsWebSocketVersion: "5.7.6", RPCVersion: 1}, nil
}
func (b *fakeBackend) Inputs(context.Context) ([]domain.OBSInput, error) {
	return []domain.OBSInput{{Name: "Mic/Aux", UUID: "mic-uuid", Kind: "wasapi_input_capture"}}, nil
}
func (b *fakeBackend) InputMute(context.Context, domain.InputRef) (*bool, error) {
	if b.muteErr != nil {
		return nil, b.muteErr
	}
	value := true
	return &value, nil
}
func (b *fakeBackend) InputAudioTracks(context.Context, domain.InputRef) (map[string]bool, error) {
	if b.tracksErr != nil {
		return nil, b.tracksErr
	}
	return map[string]bool{"track1": false, "track2": true, "track3": false, "track4": false, "track5": false, "track6": false}, nil
}
func (b *fakeBackend) ProgramScene(context.Context) (string, string, error) {
	return "Main", "scene-uuid", nil
}
func (b *fakeBackend) Scenes(context.Context) ([]domain.OBSScene, error) {
	return []domain.OBSScene{{Name: "Main", UUID: "scene-uuid"}}, nil
}
func (b *fakeBackend) SceneItems(context.Context, string) ([]domain.OBSSceneItem, error) {
	return []domain.OBSSceneItem{{ID: 7, SourceName: "Camera", SourceUUID: "camera-uuid", Enabled: false}}, nil
}
func (b *fakeBackend) SceneItemEnabled(context.Context, string, int) (*bool, error) {
	value := false
	return &value, nil
}
func (b *fakeBackend) StreamStatus(context.Context) (*bool, error) {
	value := true
	return &value, nil
}
func (b *fakeBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

func (b *fakeBackend) SceneCollection(context.Context) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.collection, nil
}

type fakeDialer struct {
	mu       sync.Mutex
	backend  *fakeBackend
	password string
	address  string
	err      error
	failures int
	calls    int
}

func (d *fakeDialer) Connect(_ context.Context, address, password string) (Backend, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	d.address = address
	d.password = password
	if d.failures > 0 {
		d.failures--
		return nil, errors.New("temporary dial failure")
	}
	if d.err != nil {
		return nil, d.err
	}
	if d.backend == nil {
		return nil, errors.New("dial failed")
	}
	return d.backend, nil
}

func testSettings() domain.Expectations {
	track := 2
	return domain.Expectations{
		Microphone:  &domain.InputRef{Name: "Mic/Aux", UUID: "mic-uuid"},
		StreamTrack: &track,
		Visual: &domain.VisualExpectation{
			Scene:       "Main",
			SceneUUID:   "scene-uuid",
			SceneItemID: "7",
			Source:      domain.InputRef{Name: "Camera", UUID: "camera-uuid"},
		},
	}
}

func newFakeManager(t *testing.T) (*Manager, *fakeBackend, *fakeDialer, *secrets.Memory) {
	t.Helper()
	backend := &fakeBackend{collection: "Default Collection"}
	dialer := &fakeDialer{backend: backend}
	secretStore := secrets.NewMemory()
	manager := NewManager(Options{
		Context:  context.Background(),
		Dialer:   dialer,
		Secrets:  secretStore,
		Now:      time.Now,
		OnUpdate: func() {},
	})
	manager.SetExpectations(testSettings())
	t.Cleanup(manager.Close)
	return manager, backend, dialer, secretStore
}

func waitConnected(t *testing.T, m *Manager) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if m.Snapshot().Status == domain.ConnectionConnected {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("OBS manager did not connect: %+v", m.Snapshot())
}

func TestOBSConnectDiscoveryObservationAndResolution(t *testing.T) {
	manager, backend, dialer, secretStore := newFakeManager(t)
	password := "obs-password-sentinel"
	if err := manager.Connect("ws://127.0.0.1:4455", &password); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, manager)
	dialer.mu.Lock()
	if dialer.password != password {
		t.Fatal("OBS password was not passed to the dialer")
	}
	dialer.mu.Unlock()
	stored, err := secretStore.Get(context.Background(), passwordKey("ws://127.0.0.1:4455"))
	if err != nil || stored != password {
		t.Fatalf("stored OBS password missing: %v", err)
	}
	discovery := manager.Discovery()
	if discovery == nil || len(discovery.Inputs) != 1 || len(discovery.Scenes) != 1 || len(discovery.Scenes[0].Items) != 1 {
		t.Fatalf("unexpected discovery: %+v", discovery)
	}
	observation := manager.Observation()
	if observation == nil || observation.Mute == nil || !*observation.Mute {
		t.Fatalf("mute unknown: %+v", observation)
	}
	if !observation.Tracks["track2"] || len(observation.Tracks) != 6 {
		t.Fatalf("track map = %+v", observation.Tracks)
	}
	if observation.VisualItemEnabled == nil || *observation.VisualItemEnabled {
		t.Fatalf("visual item = %+v", observation.VisualItemEnabled)
	}
	if observation.Streaming == nil || !*observation.Streaming {
		t.Fatalf("streaming = %+v", observation.Streaming)
	}
	if !manager.ResolutionReady(testSettings()) {
		t.Fatal("configured references did not resolve")
	}
	snapshot := manager.Snapshot()
	if snapshot.Status != domain.ConnectionConnected || snapshot.Broadcasting == nil || !*snapshot.Broadcasting || snapshot.Stale {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if err := manager.Connect("ws://127.0.0.1:4455", nil); !errors.Is(err, ErrBusy) {
		t.Fatalf("second connect error = %v, want busy", err)
	}
	_ = backend
}

func TestOBSPartialReadsRemainUnknown(t *testing.T) {
	manager, backend, _, _ := newFakeManager(t)
	backend.muteErr = errors.New("mute read failed")
	backend.tracksErr = errors.New("tracks read failed")
	if err := manager.Connect("ws://127.0.0.1:4455", func() *string { s := ""; return &s }()); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, manager)
	observation := manager.Observation()
	if observation == nil || observation.Mute != nil || observation.Tracks != nil {
		t.Fatalf("partial observation invented values: %+v", observation)
	}
	if len(observation.Partial) < 2 {
		t.Fatalf("partial fields not recorded: %+v", observation.Partial)
	}
	if manager.Snapshot().Status != domain.ConnectionConnected {
		t.Fatal("partial field read changed the connection state")
	}
}

func TestOBSDisconnectInvalidatesAndCloses(t *testing.T) {
	manager, backend, _, _ := newFakeManager(t)
	if err := manager.Connect("ws://127.0.0.1:4455", nil); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, manager)
	if err := manager.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if manager.Discovery() != nil || manager.Observation() != nil {
		t.Fatal("disconnect retained discovery/observation")
	}
	if manager.Snapshot().Status != domain.ConnectionNotConnected {
		t.Fatalf("status after disconnect = %s", manager.Snapshot().Status)
	}
	backend.mu.Lock()
	closed := backend.closed
	backend.mu.Unlock()
	if !closed {
		t.Fatal("backend was not closed on disconnect")
	}
}

func TestOBSPasswordScopedToEndpoint(t *testing.T) {
	manager, _, dialer, _ := newFakeManager(t)
	password := "endpoint-a-password"
	if err := manager.Connect("ws://127.0.0.1:4455", &password); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, manager)
	if err := manager.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Connect("ws://127.0.0.1:4456", nil); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, manager)
	dialer.mu.Lock()
	defer dialer.mu.Unlock()
	if dialer.password != "" {
		t.Fatalf("password for a different endpoint was reused: %q", dialer.password)
	}
}

func TestOBSStaleSnapshot(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	backend := &fakeBackend{}
	manager := NewManager(Options{
		Context:    context.Background(),
		Dialer:     &fakeDialer{backend: backend},
		Secrets:    secrets.NewMemory(),
		Now:        func() time.Time { return now },
		StaleAfter: 5 * time.Second,
	})
	t.Cleanup(manager.Close)
	manager.SetExpectations(testSettings())
	if err := manager.Connect("ws://127.0.0.1:4455", nil); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, manager)
	if manager.Snapshot().Stale {
		t.Fatal("fresh observation marked stale")
	}
	now = now.Add(10 * time.Second)
	if !manager.Snapshot().Stale {
		t.Fatal("old observation not marked stale")
	}
}

func TestOBSValidateEndpoint(t *testing.T) {
	for _, address := range []string{"ws://127.0.0.1:4455", "wss://[::1]:4455"} {
		if err := ValidateEndpoint(address); err != nil {
			t.Errorf("%s rejected: %v", address, err)
		}
	}
	for _, address := range []string{"ws://0.0.0.0:4455", "http://127.0.0.1:4455", "ws://localhost:4455", "ws://user:pass@127.0.0.1:4455", "ws://127.0.0.1"} {
		if err := ValidateEndpoint(address); err == nil {
			t.Errorf("%s accepted", address)
		}
	}
}

func TestOBSConnectionFailureIsSafeAndNotReady(t *testing.T) {
	manager := NewManager(Options{
		Context:  context.Background(),
		Dialer:   &fakeDialer{err: errors.New("auth failed with raw detail")},
		Secrets:  secrets.NewMemory(),
		Now:      time.Now,
		OnUpdate: func() {},
	})
	t.Cleanup(manager.Close)
	if err := manager.Connect("ws://127.0.0.1:4455", nil); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if manager.Snapshot().Status == domain.ConnectionNeedsAttention {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if manager.Snapshot().Status != domain.ConnectionNeedsAttention {
		t.Fatalf("status = %s, want needs_attention", manager.Snapshot().Status)
	}
	if strings.Contains(manager.Snapshot().Detail, "raw detail") {
		t.Fatal("raw OBS error leaked into private detail")
	}
	if manager.ResolutionReady(testSettings()) {
		t.Fatal("offline manager reported ready resolution")
	}
}

func TestOBSInitialDialRetries(t *testing.T) {
	backend := &fakeBackend{}
	dialer := &fakeDialer{backend: backend, failures: 1}
	manager := NewManager(Options{
		Context:  context.Background(),
		Dialer:   dialer,
		Secrets:  secrets.NewMemory(),
		Now:      time.Now,
		OnUpdate: func() {},
	})
	t.Cleanup(manager.Close)
	manager.SetExpectations(testSettings())
	if err := manager.Connect("ws://127.0.0.1:4455", nil); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, manager)
	dialer.mu.Lock()
	calls := dialer.calls
	dialer.mu.Unlock()
	if calls != 2 {
		t.Fatalf("dial attempts = %d, want 2", calls)
	}
}

func TestOBSResolutionRejectsRecreatedIdentity(t *testing.T) {
	manager, _, _, _ := newFakeManager(t)
	manager.mu.Lock()
	manager.discovery = &domain.OBSDiscovery{
		Inputs: []domain.OBSInput{{Name: "Mic/Aux", UUID: "different-uuid"}},
		Scenes: []domain.OBSScene{{Name: "Main", UUID: "scene-uuid", Items: []domain.OBSSceneItem{{ID: 7, SourceName: "Different Source", SourceUUID: "different-source-uuid"}}}},
	}
	manager.mu.Unlock()
	if manager.ResolutionReady(testSettings()) {
		t.Fatal("recreated identity was treated as ready")
	}
}

func TestOBSBackendSurfaceIsReadOnly(t *testing.T) {
	typ := reflect.TypeOf((*Backend)(nil)).Elem()
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		for _, forbidden := range []string{"Set", "Toggle", "Start", "Stop", "Create", "Remove", "Duplicate"} {
			if strings.HasPrefix(name, forbidden) {
				t.Fatalf("OBS backend exposes mutation method %s", name)
			}
		}
	}
}
