package obs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/desanv01/cuemender/internal/domain"
	gorilla "github.com/gorilla/websocket"
)

// R01 protocol checks use the pinned goobs backend against a local WebSocket
// server. They are local protocol fixtures, not live OBS evidence.
func startOBSProtocolServer(t *testing.T, respond func(requestID, requestType string) map[string]any) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
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
			requests.Add(1)
			_ = conn.WriteJSON(respond(request.D.RequestID, request.D.RequestType))
		}
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

func TestOBSProtocolMissingTracksStayUnknown(t *testing.T) {
	server, requests := startOBSProtocolServer(t, func(id, requestType string) map[string]any {
		return map[string]any{"op": 7, "d": map[string]any{
			"requestType": requestType, "requestId": id,
			"requestStatus": map[string]any{"result": true, "code": 100},
			"responseData":  map[string]any{},
		}}
	})
	backend, err := (GoobsDialer{}).Connect(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	tracks, err := backend.InputAudioTracks(context.Background(), domain.InputRef{Name: "Mic"})
	if err != nil {
		t.Fatal(err)
	}
	if completeTracks(tracks) {
		t.Fatalf("missing wire fields became complete: %v", tracks)
	}
	if requests.Load() == 0 {
		t.Fatal("no outbound OBS request was observed")
	}
}

func TestOBSProtocolErrorIsReturned(t *testing.T) {
	server, _ := startOBSProtocolServer(t, func(id, requestType string) map[string]any {
		return map[string]any{"op": 7, "d": map[string]any{
			"requestType": requestType, "requestId": id,
			"requestStatus": map[string]any{"result": false, "code": 600, "comment": "synthetic provider failure"},
		}}
	})
	backend, err := (GoobsDialer{}).Connect(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if _, err := backend.StreamStatus(context.Background()); err == nil {
		t.Fatal("OBS protocol error was not returned")
	}
}

func TestOBSReadOnlyOutboundAllowlist(t *testing.T) {
	allowed := map[string]bool{
		"GetVersion": true, "GetInputList": true, "GetInputMute": true,
		"GetInputAudioTracks": true, "GetCurrentProgramScene": true,
		"GetSceneList": true, "GetSceneItemList": true, "GetSceneItemEnabled": true,
		"GetStreamStatus": true, "GetSceneCollectionList": true,
	}
	var unexpected atomic.Value
	server, _ := startOBSProtocolServer(t, func(id, requestType string) map[string]any {
		if !allowed[requestType] {
			unexpected.Store(requestType)
		}
		data := map[string]any{}
		switch requestType {
		case "GetVersion":
			data = map[string]any{"obsVersion": "32.1.1", "obsWebSocketVersion": "5.7.6", "rpcVersion": 1}
		case "GetInputList":
			data = map[string]any{"inputs": []any{}}
		case "GetInputMute":
			data = map[string]any{"inputMuted": false}
		case "GetInputAudioTracks":
			data = map[string]any{"inputAudioTracks": map[string]bool{"1": false, "2": true, "3": false, "4": false, "5": false, "6": false}}
		case "GetCurrentProgramScene":
			data = map[string]any{"sceneName": "Main", "sceneUuid": "scene-uuid"}
		case "GetSceneList":
			data = map[string]any{"scenes": []any{}}
		case "GetSceneItemList":
			data = map[string]any{"sceneItems": []any{}}
		case "GetSceneItemEnabled":
			data = map[string]any{"sceneItemEnabled": false}
		case "GetStreamStatus":
			data = map[string]any{"outputActive": false}
		case "GetSceneCollectionList":
			data = map[string]any{"currentSceneCollectionName": "Main Collection"}
		}
		return map[string]any{"op": 7, "d": map[string]any{
			"requestType": requestType, "requestId": id,
			"requestStatus": map[string]any{"result": true, "code": 100},
			"responseData":  data,
		}}
	})
	backend, err := (GoobsDialer{}).Connect(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if _, err := backend.Version(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Inputs(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.InputMute(context.Background(), domain.InputRef{Name: "Mic"}); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.InputAudioTracks(context.Background(), domain.InputRef{Name: "Mic"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := backend.ProgramScene(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Scenes(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.SceneItems(context.Background(), "scene-uuid"); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.SceneItemEnabled(context.Background(), "scene-uuid", 7); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.StreamStatus(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.(*goobsBackend).SceneCollection(context.Background()); err != nil {
		t.Fatal(err)
	}
	if value := unexpected.Load(); value != nil {
		t.Fatalf("non-allowlisted OBS request %v", value)
	}
}
