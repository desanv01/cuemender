package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/obs"
	"github.com/desanv01/cuemender/internal/twitch"
	gorilla "github.com/gorilla/websocket"
)

var (
	p03ObsMu       sync.Mutex
	p03ObsRequests []string
)

func recordOBSRequest(name string) {
	p03ObsMu.Lock()
	p03ObsRequests = append(p03ObsRequests, name)
	p03ObsMu.Unlock()
}

func localOBSServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&gorilla.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteJSON(map[string]any{"op": 0, "d": map[string]any{"obsWebSocketVersion": "5.7.3", "rpcVersion": 1}})
		var identify map[string]any
		if conn.ReadJSON(&identify) != nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"op": 2, "d": map[string]any{"negotiatedRpcVersion": 1}})
		for {
			var request struct {
				Op int `json:"op"`
				D  struct {
					Type string `json:"requestType"`
					ID   string `json:"requestId"`
				} `json:"d"`
			}
			if conn.ReadJSON(&request) != nil {
				return
			}
			data := map[string]any{}
			recordOBSRequest(request.D.Type)
			switch request.D.Type {
			case "GetVersion":
				data = map[string]any{"obsVersion": "32.1.1", "obsWebSocketVersion": "5.7.3", "rpcVersion": 1}
			case "GetInputList":
				data = map[string]any{"inputs": []any{map[string]any{"inputName": "Test Mic", "inputUuid": "mic-uuid", "inputKind": "wasapi_input_capture"}}}
			case "GetSceneList":
				data = map[string]any{"scenes": []any{map[string]any{"sceneName": "Main", "sceneUuid": "scene-uuid"}}}
			case "GetSceneItemList":
				data = map[string]any{"sceneItems": []any{map[string]any{"sceneItemId": 7, "sourceName": "Camera", "sourceUuid": "camera-uuid", "sceneItemEnabled": true}}}
			case "GetSceneCollectionList":
				data = map[string]any{"currentSceneCollectionName": "Collection"}
			case "GetCurrentProgramScene":
				data = map[string]any{"sceneName": "Main", "sceneUuid": "scene-uuid"}
			case "GetInputMute":
				data = map[string]any{"inputMuted": true}
			case "GetInputAudioTracks":
				data = map[string]any{"inputAudioTracks": map[string]bool{"1": false, "2": false, "3": false, "4": false, "5": false, "6": false}}
			case "GetSceneItemEnabled":
				data = map[string]any{"sceneItemEnabled": true}
			case "GetStreamStatus":
				data = map[string]any{"outputActive": false}
			default:
				data = map[string]any{}
			}
			_ = conn.WriteJSON(map[string]any{"op": 7, "d": map[string]any{
				"requestType": request.D.Type, "requestId": request.D.ID,
				"requestStatus": map[string]any{"result": true, "code": 100},
				"responseData":  data,
			}})
		}
	}))
	t.Cleanup(server.Close)
	return server
}

type protocolChat struct {
	keepalive bool
	welcome   chan string
	help      chan string
	chat      chan string
	replies   chan string
	handlerWG sync.WaitGroup
}

func localTwitchServer(t *testing.T, chat *protocolChat) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	var sent []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device":
			_ = json.NewEncoder(w).Encode(map[string]any{"device_code": "device-code", "user_code": "USER-CODE", "verification_uri": "https://www.twitch.tv/activate", "expires_in": 600, "interval": 1})
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-token", "refresh_token": "refresh-token", "expires_in": 3600, "scope": []string{twitch.ScopeReadChat, twitch.ScopeWriteChat}})
		case "/validate":
			_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "abc123", "login": "broadcaster", "user_id": "user-1", "scopes": []string{twitch.ScopeReadChat, twitch.ScopeWriteChat}, "expires_in": 3600})
		case "/helix/eventsub/subscriptions":
			w.WriteHeader(http.StatusAccepted)
		case "/helix/chat/messages":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			if msg, ok := body["message"].(string); ok {
				sent = append(sent, msg)
				select {
				case chat.replies <- msg:
				default:
				}
			}
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"is_sent": true}}})
		default:
			if strings.HasSuffix(r.URL.Path, "/ws") {
				conn, err := (&gorilla.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					return
				}
				chat.handlerWG.Add(1)
				defer chat.handlerWG.Done()
				defer conn.Close()
				_ = conn.WriteJSON(map[string]any{"metadata": map[string]any{"message_id": "welcome", "message_type": "session_welcome", "message_timestamp": "2026-11-01T00:00:00Z"}, "payload": map[string]any{"session": map[string]any{"id": "session", "status": "connected", "keepalive_timeout_seconds": 30}}})
				chat.welcome <- "ok"
				readDone := make(chan struct{})
				go func() {
					for {
						if _, _, err := conn.ReadMessage(); err != nil {
							close(readDone)
							return
						}
					}
				}()
				defer func() {
					_ = conn.Close()
					<-readDone
				}()
				select {
				case <-chat.help:
					_ = conn.WriteJSON(map[string]any{"metadata": map[string]any{"message_id": "help-1", "message_type": "notification", "message_timestamp": "2026-11-01T00:00:00Z"}, "payload": map[string]any{
						"subscription": map[string]any{"type": "channel.chat.message", "version": "1"},
						"event":        map[string]any{"broadcaster_user_id": "user-1", "chatter_user_id": "user-2", "message_id": "help-message", "message_type": "text", "message": map[string]any{"text": "!streamhelp"}},
					}})
				case <-readDone:
					return
				case <-time.After(5 * time.Second):
					return
				}
				select {
				case <-chat.chat:
					_ = conn.WriteJSON(map[string]any{"metadata": map[string]any{"message_id": "delivery-1", "message_type": "notification", "message_timestamp": "2026-11-01T00:00:01Z"}, "payload": map[string]any{
						"subscription": map[string]any{"type": "channel.chat.message", "version": "1"},
						"event":        map[string]any{"broadcaster_user_id": "user-1", "chatter_user_id": "user-2", "message_id": "message-1", "message_type": "text", "message": map[string]any{"text": "!issueaudio"}},
					}})
				case <-readDone:
					return
				case <-time.After(5 * time.Second):
					return
				}
				if chat.keepalive {
					ticker := time.NewTicker(5 * time.Second)
					defer ticker.Stop()
					for {
						select {
						case <-readDone:
							return
						case <-ticker.C:
							if err := conn.WriteJSON(map[string]any{"metadata": map[string]any{"message_id": fmt.Sprintf("keepalive-%d", time.Now().UnixNano()), "message_type": "session_keepalive", "message_timestamp": time.Now().UTC()}}); err != nil {
								return
							}
						}
					}
				}
				<-readDone
				return
			}
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func waitStatus(t *testing.T, f func() string, want string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if f() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("status never became %s (last %s)", want, f())
}

func TestP03ActualAppProtocolReportAndReply(t *testing.T) {
	p03ObsMu.Lock()
	p03ObsRequests = nil
	p03ObsMu.Unlock()
	obsServer := localOBSServer(t)
	chat := &protocolChat{welcome: make(chan string, 1), help: make(chan string, 1), chat: make(chan string, 1), replies: make(chan string, 1)}
	twitchServer := localTwitchServer(t, chat)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(twitchServer.URL, "http") + "/ws"
	a, err := newWithDependencies(ctx, Options{DataDir: t.TempDir(), ListenAddress: fmt.Sprintf("127.0.0.1:%d", port), NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard}, appDependencies{
		configureTwitch: func(opts *twitch.Options) {
			opts.OAuth = twitch.NewOAuth(twitchServer.Client(), twitch.Endpoints{Device: twitchServer.URL + "/device", Token: twitchServer.URL + "/token", Validate: twitchServer.URL + "/validate", Helix: twitchServer.URL + "/helix", EventSub: wsURL}, time.Now)
			opts.Dialer = twitch.CoderDialer{HTTP: &http.Client{}}
			opts.Endpoints = twitch.Endpoints{Device: twitchServer.URL + "/device", Token: twitchServer.URL + "/token", Validate: twitchServer.URL + "/validate", Helix: twitchServer.URL + "/helix", EventSub: wsURL}
			opts.ClientID = "abc123"
		},
		configureOBS: func(opts *obs.Options) { opts.Dialer = obs.GoobsDialer{} },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	runDone := make(chan error, 1)
	go func() { runDone <- a.Run(ctx) }()
	waitHealthy(t, a.BaseURL())
	track := 2
	settings, err := a.SaveSettings(ctx, 0, domain.Expectations{
		Microphone:  &domain.InputRef{Name: "Test Mic", UUID: "mic-uuid", Context: "Collection"},
		StreamTrack: &track,
		Visual:      &domain.VisualExpectation{Scene: "Main", SceneUUID: "scene-uuid", SceneItemID: "7", Source: domain.InputRef{Name: "Camera", UUID: "camera-uuid", Context: "Collection"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.OBSConnect(ctx, "ws"+strings.TrimPrefix(obsServer.URL, "http"), nil); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, func() string { return a.obsMgr.Snapshot().Status }, domain.ConnectionConnected)
	if err := a.TwitchConnect(ctx, "abc123"); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, func() string { return a.twitchMgr.Snapshot().Status }, domain.ConnectionConnected)
	chat.help <- "go"
	select {
	case help := <-chat.replies:
		if !strings.Contains(help, "streamhelp") && !strings.Contains(help, "CueMender") {
			t.Fatalf("unexpected idle help reply %q", help)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("idle !streamhelp did not receive a reply")
	}
	if _, err := a.StartMonitoring(ctx, settings.Revision); err != nil {
		t.Fatal(err)
	}
	select {
	case <-chat.welcome:
	case <-time.After(time.Second):
		t.Fatal("EventSub welcome not observed")
	}
	chat.chat <- "go"
	deadline := time.Now().Add(8 * time.Second)
	var found bool
	for time.Now().Before(deadline) {
		_, list := a.coordinator.Snapshot()
		for _, inc := range list {
			if len(inc.Findings) > 0 {
				found = true
			}
		}
		if found {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !found {
		t.Fatal("actual EventSub report did not create a persisted finding")
	}
	persisted, err := a.store.ListIncidents(ctx, 10)
	if err != nil || len(persisted) == 0 || len(persisted[0].Findings) == 0 {
		t.Fatalf("finding was not durable in SQLite: incidents=%d err=%v", len(persisted), err)
	}
	// The reply is bounded; allow the 3-second aggregate interval.
	select {
	case reply := <-chat.replies:
		if !strings.Contains(reply, "received") && !strings.Contains(reply, "added") {
			t.Fatalf("unexpected actual reply %q", reply)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("actual Helix reply was not sent")
	}
	cancel()
	select {
	case <-runDone:
	case <-time.After(3 * time.Second):
		t.Fatal("app did not stop")
	}
	a.Close()
	p03ObsMu.Lock()
	requests := append([]string(nil), p03ObsRequests...)
	p03ObsMu.Unlock()
	allowed := map[string]bool{
		"GetVersion": true, "GetInputList": true, "GetSceneList": true,
		"GetSceneItemList": true, "GetSceneCollectionList": true,
		"GetCurrentProgramScene": true, "GetInputMute": true,
		"GetInputAudioTracks": true, "GetSceneItemEnabled": true,
		"GetStreamStatus": true,
	}
	for _, request := range requests {
		if !allowed[request] {
			t.Fatalf("unexpected OBS request outside read-only allowlist: %s", request)
		}
	}
	handlersDone := make(chan struct{})
	go func() { chat.handlerWG.Wait(); close(handlersDone) }()
	select {
	case <-handlersDone:
	case <-time.After(2 * time.Second):
		t.Fatal("upgraded EventSub handler/socket did not join")
	}
}
