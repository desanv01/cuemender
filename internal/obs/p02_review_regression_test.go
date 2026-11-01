// Promoted from Codex's review-only regression set. The behavioral assertions
// are unchanged; these now run in the ordinary suite.
package obs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/secrets"
	gorilla "github.com/gorilla/websocket"
)

func TestReviewOBSRealWireTrackMap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&gorilla.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteJSON(map[string]any{"op": 0, "d": map[string]any{"obsWebSocketVersion": "5.5.0", "rpcVersion": 1}})
		var identify map[string]any
		if conn.ReadJSON(&identify) != nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"op": 2, "d": map[string]any{"negotiatedRpcVersion": 1}})
		for {
			var request struct {
				Op int `json:"op"`
				D  struct {
					RequestType string `json:"requestType"`
					RequestID   string `json:"requestId"`
				} `json:"d"`
			}
			if conn.ReadJSON(&request) != nil {
				return
			}
			if request.D.RequestType != "GetInputAudioTracks" {
				t.Errorf("unexpected outbound request %s", request.D.RequestType)
				return
			}
			_ = conn.WriteJSON(map[string]any{"op": 7, "d": map[string]any{
				"requestType": request.D.RequestType, "requestId": request.D.RequestID,
				"requestStatus": map[string]any{"result": true, "code": 100},
				"responseData":  map[string]any{"inputAudioTracks": map[string]bool{"1": false, "2": true, "3": false, "4": false, "5": false, "6": true}},
			}})
		}
	}))
	defer server.Close()
	backend, err := (GoobsDialer{}).Connect(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	tracks, err := backend.InputAudioTracks(context.Background(), domain.InputRef{Name: "Review mic"})
	if err != nil {
		t.Fatal(err)
	}
	if !completeTracks(tracks) {
		t.Fatalf("real OBS wire keys are not normalized into a complete domain map: %v", tracks)
	}
}

func TestReviewOBSReuseSavedPasswordAfterRestart(t *testing.T) {
	manager, _, dialer, secretStore := newFakeManager(t)
	endpoint := "ws://127.0.0.1:4455"
	if err := secretStore.Set(context.Background(), passwordKey(endpoint), "synthetic-password"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Connect(endpoint, nil); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, manager)
	dialer.mu.Lock()
	password := dialer.password
	dialer.mu.Unlock()
	if password != "synthetic-password" {
		t.Fatal("fresh manager did not load the saved password for the configured endpoint")
	}
}

type reviewOBSBackend struct {
	fakeBackend
	changed atomic.Bool
	broken  atomic.Bool
	polls   atomic.Int32
}

func (b *reviewOBSBackend) Inputs(ctx context.Context) ([]domain.OBSInput, error) {
	inputs, err := b.fakeBackend.Inputs(ctx)
	if b.changed.Load() {
		inputs[0].UUID = "recreated-mic-uuid"
	}
	return inputs, err
}
func (b *reviewOBSBackend) InputMute(ctx context.Context, ref domain.InputRef) (*bool, error) {
	if b.broken.Load() {
		return nil, errors.New("socket closed")
	}
	return b.fakeBackend.InputMute(ctx, ref)
}
func (b *reviewOBSBackend) InputAudioTracks(ctx context.Context, ref domain.InputRef) (map[string]bool, error) {
	if b.broken.Load() {
		return nil, errors.New("socket closed")
	}
	return b.fakeBackend.InputAudioTracks(ctx, ref)
}
func (b *reviewOBSBackend) ProgramScene(ctx context.Context) (string, string, error) {
	if b.broken.Load() {
		return "", "", errors.New("socket closed")
	}
	return b.fakeBackend.ProgramScene(ctx)
}
func (b *reviewOBSBackend) SceneItemEnabled(ctx context.Context, scene string, id int) (*bool, error) {
	if b.broken.Load() {
		return nil, errors.New("socket closed")
	}
	return b.fakeBackend.SceneItemEnabled(ctx, scene, id)
}
func (b *reviewOBSBackend) StreamStatus(ctx context.Context) (*bool, error) {
	defer b.polls.Add(1)
	if b.broken.Load() {
		return nil, errors.New("socket closed")
	}
	return b.fakeBackend.StreamStatus(ctx)
}

type reviewOBSDialer struct{ backend Backend }

func (d reviewOBSDialer) Connect(context.Context, string, string) (Backend, error) {
	return d.backend, nil
}
func reviewConnectedOBS(t *testing.T) (*Manager, *reviewOBSBackend) {
	t.Helper()
	b := &reviewOBSBackend{}
	m := NewManager(Options{Dialer: reviewOBSDialer{b}, Secrets: secrets.NewMemory(), PollInterval: 10 * time.Millisecond})
	m.SetExpectations(testSettings())
	t.Cleanup(m.Close)
	password := ""
	if err := m.Connect("ws://127.0.0.1:4455", &password); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, m)
	return m, b
}
func reviewWaitPolls(t *testing.T, b *reviewOBSBackend, target int32) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for b.polls.Load() < target && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if b.polls.Load() < target {
		t.Fatal("poll fixture did not advance")
	}
}
func TestReviewOBSRejectsRecreatedInputDuringConnection(t *testing.T) {
	m, b := reviewConnectedOBS(t)
	target := b.polls.Load() + 3
	b.changed.Store(true)
	reviewWaitPolls(t, b, target)
	if m.ResolutionReady(testSettings()) {
		t.Fatal("same-name source recreation leaves stale discovery marked resolved")
	}
}
func TestReviewOBSClosedSocketNotFreshConnected(t *testing.T) {
	m, b := reviewConnectedOBS(t)
	target := b.polls.Load() + 3
	b.broken.Store(true)
	reviewWaitPolls(t, b, target)
	snapshot := m.Snapshot()
	if snapshot.Status == domain.ConnectionConnected && !snapshot.Stale {
		t.Fatal("all OBS reads fail but the manager still reports connected and fresh")
	}
}
