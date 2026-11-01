package obs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/gorilla/websocket"
)

func TestDiscoveryAdmissionAndCloseJoin(t *testing.T) {
	b := &reviewR03SlowInputs{fakeBackend: fakeBackend{collection: "A"}, entered: make(chan struct{}), release: make(chan struct{})}
	m := NewManager(Options{})
	m.mu.Lock()
	m.backend, m.gen, m.conn.Status = b, 1, domain.ConnectionConnected
	m.mu.Unlock()
	result := make(chan error, 1)
	go func() { _, err := m.RefreshDiscovery(context.Background()); result <- err }()
	<-b.entered
	if _, err := m.RefreshDiscovery(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatalf("overlapping refresh: %v", err)
	}
	closed := make(chan struct{})
	go func() { m.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not join admitted refresh")
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("canceled refresh succeeded")
		}
	default:
		t.Fatal("refresh not joined")
	}
	if _, err := m.RefreshDiscovery(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("admitted after Close: %v", err)
	}
}

func TestConnectCloseRegistrationIsSerialized(t *testing.T) {
	for i := 0; i < 50; i++ {
		b := &fakeBackend{collection: "A"}
		m := NewManager(Options{Dialer: reviewOBSDialer{b}, PollInterval: time.Hour})
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; _ = m.Connect("ws://127.0.0.1:4455", nil) }()
		go func() { defer wg.Done(); <-start; m.Close() }()
		close(start)
		wg.Wait()
		if err := m.Connect("ws://127.0.0.1:4455", nil); !errors.Is(err, ErrBusy) {
			t.Fatalf("closed manager admitted connect: %v", err)
		}
	}
}

func TestOBSSettingsAndSnapshotsOwnPointers(t *testing.T) {
	m := NewManager(Options{})
	defer m.Close()
	settings := testSettings()
	m.SetSettings(1, settings)
	settings.Microphone.Name = "changed by caller"
	*settings.StreamTrack = 6
	settings.Visual.Scene = "changed by caller"
	if m.settings.Microphone.Name != "Mic/Aux" || *m.settings.StreamTrack != 2 || m.settings.Visual.Scene != "Main" {
		t.Fatal("settings aliases caller pointers")
	}
	value := true
	m.observation = &domain.OBSObservation{ConfigRevision: 1, Mute: &value, Streaming: &value, VisualItemEnabled: &value, Microphone: &domain.InputRef{Name: "Mic"}, ObservedAt: time.Now()}
	copy := m.Observation()
	*copy.Mute, *copy.Streaming, *copy.VisualItemEnabled, copy.Microphone.Name = false, false, false, "changed"
	conn := m.Snapshot()
	*conn.Broadcasting = false
	if !*m.observation.Mute || !*m.observation.Streaming || !*m.observation.VisualItemEnabled || m.observation.Microphone.Name != "Mic" {
		t.Fatal("snapshot aliases observation pointers")
	}
}

type collectionSequence struct {
	fakeBackend
	values []string
	calls  int
}

func (b *collectionSequence) SceneCollection(context.Context) (string, error) {
	value := b.values[b.calls]
	b.calls++
	return value, nil
}

func TestReadCycleRejectsContextSwitchThroughFinalObservation(t *testing.T) {
	for _, values := range [][]string{{"", "A", "A"}, {"A", "B", "B"}, {"A", "A", "B"}} {
		b := &collectionSequence{values: values}
		m := NewManager(Options{})
		m.SetSettings(1, testSettings())
		d, o, err := m.readCycle(context.Background(), 0, b)
		m.Close()
		if err != nil {
			t.Fatal(err)
		}
		if d.CollectionKnown || o.SceneCollection != "" || o.Mute != nil || o.Tracks != nil || o.VisualItemEnabled != nil {
			t.Fatalf("ambiguous cycle retained evidence: discovery=%+v observation=%+v", d, o)
		}
	}
}

func TestReadCycleCapturesSettingsBeforeDiscovery(t *testing.T) {
	b := &reviewR03SlowInputs{fakeBackend: fakeBackend{collection: "A"}, entered: make(chan struct{}), release: make(chan struct{})}
	m := NewManager(Options{})
	defer m.Close()
	m.SetSettings(1, testSettings())
	type result struct {
		d   *domain.OBSDiscovery
		o   *domain.OBSObservation
		err error
	}
	done := make(chan result, 1)
	go func() { d, o, err := m.readCycle(context.Background(), 0, b); done <- result{d, o, err} }()
	<-b.entered
	settings := testSettings()
	settings.Microphone.Name = "New microphone"
	m.SetSettings(2, settings)
	close(b.release)
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.d.ConfigRevision != 1 || got.o.ConfigRevision != 1 || got.o.Microphone.Name != "Mic/Aux" {
		t.Fatalf("cycle mixed settings: %+v %+v", got.d, got.o)
	}
}

func TestReadinessRejectsDiscoveryGenerationRevisionAndUnknownContext(t *testing.T) {
	m, _, _, _ := newFakeManager(t)
	if err := m.Connect("ws://127.0.0.1:4455", nil); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, m)
	for _, mismatch := range []string{"generation", "revision", "context"} {
		m.mu.Lock()
		original := *m.discovery
		switch mismatch {
		case "generation":
			m.discovery.Generation++
		case "revision":
			m.discovery.ConfigRevision++
		case "context":
			m.discovery.CollectionKnown = false
		}
		m.mu.Unlock()
		if _, _, _, ready := m.SnapshotBundle(domain.Settings{Expectations: testSettings()}); ready {
			t.Fatalf("ready with wrong %s", mismatch)
		}
		m.mu.Lock()
		*m.discovery = original
		m.mu.Unlock()
	}
}

func TestVisualSceneRecreationDoesNotResolve(t *testing.T) {
	settings := testSettings()
	settings.Visual.SceneUUID = "old-scene"
	d := &domain.OBSDiscovery{CollectionKnown: true, SceneCollection: "A", Inputs: []domain.OBSInput{{Name: "Mic/Aux", UUID: "mic-uuid"}}, Scenes: []domain.OBSScene{{Name: "Main", UUID: "new-scene", Items: []domain.OBSSceneItem{{ID: 7, SourceName: "Camera", SourceUUID: "camera-uuid"}}}}}
	if resolutionReady(d, settings) {
		t.Fatal("same-name recreated scene resolved under old UUID")
	}
}

func TestNameOnlySelectionNeedsReselectionAfterContextLoss(t *testing.T) {
	m := NewManager(Options{})
	defer m.Close()
	settings := testSettings()
	settings.Microphone.UUID = ""
	m.SetSettings(1, settings)
	b := &fakeBackend{collection: "A"}
	d, o, err := m.readCycle(context.Background(), 0, b)
	if err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.discovery, m.observation = d, o
	m.conn.Status = domain.ConnectionConnected
	m.mu.Unlock()
	if _, _, _, ready := m.SnapshotBundle(domain.Settings{Revision: 1, Expectations: settings}); ready {
		t.Fatal("persisted name-only selection auto-resolved without live revalidation")
	}
	m.SetSettings(2, settings)
	d, o, _ = m.readCycle(context.Background(), 0, b)
	m.mu.Lock()
	m.discovery, m.observation = d, o
	m.mu.Unlock()
	if _, _, _, ready := m.SnapshotBundle(domain.Settings{Revision: 2, Expectations: settings}); !ready {
		t.Fatal("explicit live reselection did not resolve")
	}
	b.collection = ""
	d, o, _ = m.readCycle(context.Background(), 0, b)
	m.mu.Lock()
	m.discovery, m.observation = d, o
	m.mu.Unlock()
	b.collection = "A"
	d, o, _ = m.readCycle(context.Background(), 0, b)
	m.mu.Lock()
	m.discovery, m.observation = d, o
	m.mu.Unlock()
	if _, _, _, ready := m.SnapshotBundle(domain.Settings{Revision: 2, Expectations: settings}); ready {
		t.Fatal("name-only selection regained authority after context loss")
	}
}

func TestReconnectHelloCancellationJoinsActualAdapter(t *testing.T) {
	var connections atomic.Int32
	reconnecting, replacementDone := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if connections.Add(1) > 1 {
			defer close(replacementDone)
			close(reconnecting)
			_, _, _ = conn.ReadMessage()
			return
		}
		_ = conn.WriteJSON(map[string]any{"op": 0, "d": map[string]any{"rpcVersion": 1}})
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"op": 2, "d": map[string]any{"negotiatedRpcVersion": 1}})
		inputReads := 0
		for {
			var request struct {
				D struct {
					Type string `json:"requestType"`
					ID   string `json:"requestId"`
				} `json:"d"`
			}
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			data := map[string]any{}
			switch request.D.Type {
			case "GetVersion":
				data = map[string]any{"obsVersion": "32.1.1", "obsWebSocketVersion": "5.7.6", "rpcVersion": 1}
			case "GetSceneCollectionList":
				data = map[string]any{"currentSceneCollectionName": "A"}
			case "GetInputList":
				inputReads++
				if inputReads > 1 {
					return
				}
				data = map[string]any{"inputs": []any{}}
			case "GetSceneList":
				data = map[string]any{"scenes": []any{}}
			case "GetCurrentProgramScene":
				data = map[string]any{"sceneName": "Main", "sceneUuid": "main"}
			case "GetStreamStatus":
				data = map[string]any{"outputActive": false}
			default:
				t.Errorf("unexpected request: %s", request.D.Type)
				return
			}
			if err := conn.WriteJSON(map[string]any{"op": 7, "d": map[string]any{"requestType": request.D.Type, "requestId": request.D.ID, "requestStatus": map[string]any{"result": true, "code": 100}, "responseData": data}}); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	m := NewManager(Options{Dialer: GoobsDialer{}, PollInterval: 10 * time.Millisecond})
	defer m.Close()
	if err := m.Connect("ws"+strings.TrimPrefix(server.URL, "http"), nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reconnecting:
	case <-time.After(2 * time.Second):
		t.Fatal("adapter did not reconnect after EOF")
	}
	done := make(chan struct{})
	go func() { _ = m.Disconnect(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("reconnect handshake not joined")
	}
	select {
	case <-replacementDone:
	case <-time.After(time.Second):
		t.Fatal("replacement socket remains")
	}
}

func TestCanceledDiscoveryImmediatelyInvalidatesActualTransport(t *testing.T) {
	var connections atomic.Int32
	blocked, peerDone := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if connections.Add(1) > 1 {
			_, _, _ = conn.ReadMessage()
			return
		}
		defer close(peerDone)
		_ = conn.WriteJSON(map[string]any{"op": 0, "d": map[string]any{"rpcVersion": 1}})
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"op": 2, "d": map[string]any{"negotiatedRpcVersion": 1}})
		inputReads := 0
		for {
			var request struct {
				D struct {
					Type string `json:"requestType"`
					ID   string `json:"requestId"`
				} `json:"d"`
			}
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			data := map[string]any{}
			switch request.D.Type {
			case "GetVersion":
				data = map[string]any{"obsVersion": "32.1.1", "obsWebSocketVersion": "5.7.6", "rpcVersion": 1}
			case "GetSceneCollectionList":
				data = map[string]any{"currentSceneCollectionName": "A"}
			case "GetInputList":
				inputReads++
				if inputReads > 1 {
					close(blocked)
					_, _, _ = conn.ReadMessage()
					return
				}
				data = map[string]any{"inputs": []any{map[string]any{"inputName": "Mic", "inputUuid": "mic"}}}
			case "GetInputMute":
				data = map[string]any{"inputMuted": false}
			case "GetInputAudioTracks":
				data = map[string]any{"inputAudioTracks": map[string]bool{"1": true, "2": false, "3": false, "4": false, "5": false, "6": false}}
			case "GetSceneList":
				data = map[string]any{"scenes": []any{}}
			case "GetCurrentProgramScene":
				data = map[string]any{"sceneName": "Main", "sceneUuid": "main"}
			case "GetStreamStatus":
				data = map[string]any{"outputActive": false}
			default:
				t.Errorf("unexpected request: %s", request.D.Type)
				return
			}
			if err := conn.WriteJSON(map[string]any{"op": 7, "d": map[string]any{"requestType": request.D.Type, "requestId": request.D.ID,
				"requestStatus": map[string]any{"result": true, "code": 100}, "responseData": data}}); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	m := NewManager(Options{Dialer: GoobsDialer{}, PollInterval: time.Hour, StaleAfter: time.Hour})
	defer m.Close()
	track := 1
	settings := domain.Settings{Revision: 1, Expectations: domain.Expectations{Microphone: &domain.InputRef{Name: "Mic", UUID: "mic", Context: "A"}, StreamTrack: &track}}
	m.SetSettings(settings.Revision, settings.Expectations)
	if err := m.Connect("ws"+strings.TrimPrefix(server.URL, "http"), nil); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, m)
	before, _, _, ready := m.SnapshotBundle(settings)
	if !ready {
		t.Fatal("fixture failed to establish ready evidence")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := m.RefreshDiscovery(ctx); done <- err }()
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("fresh Inputs did not block")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled discovery succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled discovery not joined")
	}
	after, _, observation, ready := m.SnapshotBundle(settings)
	if ready || !after.Stale || after.Generation <= before.Generation || observation != nil {
		t.Fatalf("closed SDK transport retained ready evidence until poll: ready=%v before=%+v after=%+v obs=%+v", ready, before, after, observation)
	}
	select {
	case <-peerDone:
	case <-time.After(time.Second):
		t.Fatal("original socket remains open")
	}
}
