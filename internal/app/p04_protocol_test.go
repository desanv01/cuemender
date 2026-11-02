package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/obs"
	"github.com/desanv01/cuemender/internal/twitch"
	gorilla "github.com/gorilla/websocket"
)

// Isolated P04 protocol fixture: P03's read-only allowlist remains unchanged.
type p04OBSFixture struct {
	mu     sync.Mutex
	tracks map[string]bool
	writes int
	bad    bool
}

func p04OBSServer(t *testing.T) (*httptest.Server, *p04OBSFixture) {
	t.Helper()
	fixture := &p04OBSFixture{tracks: map[string]bool{"1": true, "2": false, "3": false, "4": true, "5": false, "6": true}}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&gorilla.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteJSON(map[string]any{"op": 0, "d": map[string]any{"obsWebSocketVersion": "5.7.3", "rpcVersion": 1}})
		var identify any
		if conn.ReadJSON(&identify) != nil {
			return
		}
		_ = conn.WriteJSON(map[string]any{"op": 2, "d": map[string]any{"negotiatedRpcVersion": 1}})
		for {
			var request struct {
				D struct {
					Type string `json:"requestType"`
					ID   string `json:"requestId"`
					Data struct {
						InputUUID string          `json:"inputUuid"`
						Tracks    map[string]bool `json:"inputAudioTracks"`
					} `json:"requestData"`
				} `json:"d"`
			}
			if conn.ReadJSON(&request) != nil {
				return
			}
			data := map[string]any{}
			fixture.mu.Lock()
			switch request.D.Type {
			case "GetVersion":
				data = map[string]any{"obsVersion": "32.1.1", "obsWebSocketVersion": "5.7.3", "rpcVersion": 1}
			case "GetInputList":
				data = map[string]any{"inputs": []any{map[string]any{"inputName": "P04 private mic canary", "inputUuid": "mic-uuid", "inputKind": "wasapi_input_capture"}}}
			case "GetSceneList":
				data = map[string]any{"scenes": []any{map[string]any{"sceneName": "Main", "sceneUuid": "scene-uuid"}}}
			case "GetSceneItemList":
				data = map[string]any{"sceneItems": []any{}}
			case "GetSceneCollectionList":
				data = map[string]any{"currentSceneCollectionName": "Collection"}
			case "GetCurrentProgramScene":
				data = map[string]any{"sceneName": "Main", "sceneUuid": "scene-uuid"}
			case "GetInputMute":
				data = map[string]any{"inputMuted": true}
			case "GetInputAudioTracks":
				tracks := map[string]bool{}
				for k, v := range fixture.tracks {
					tracks[k] = v
				}
				data = map[string]any{"inputAudioTracks": tracks}
			case "GetStreamStatus":
				data = map[string]any{"outputActive": false}
			case "SetInputAudioTracks":
				fixture.writes++
				if request.D.Data.InputUUID != "mic-uuid" || len(request.D.Data.Tracks) != 6 {
					fixture.bad = true
				}
				fixture.tracks = request.D.Data.Tracks
			default:
				fixture.bad = true
			}
			fixture.mu.Unlock()
			_ = conn.WriteJSON(map[string]any{"op": 7, "d": map[string]any{"requestType": request.D.Type, "requestId": request.D.ID, "requestStatus": map[string]any{"result": true, "code": 100}, "responseData": data}})
		}
	}))
	t.Cleanup(s.Close)
	return s, fixture
}

func TestP04ActualAppPrivateApprovalProtocolFlow(t *testing.T) {
	obsServer, fixture := p04OBSServer(t)
	chat := &protocolChat{keepalive: true, welcome: make(chan string, 1), help: make(chan string, 1), chat: make(chan string, 1), replies: make(chan string, 10)}
	tw := localTwitchServer(t, chat)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ws := "ws" + strings.TrimPrefix(tw.URL, "http") + "/ws"
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	a, err := newWithDependencies(ctx, Options{DataDir: t.TempDir(), ListenAddress: "127.0.0.1:" + strconv.Itoa(port), NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard}, appDependencies{configureTwitch: func(o *twitch.Options) {
		o.Endpoints = twitch.Endpoints{Device: tw.URL + "/device", Token: tw.URL + "/token", Validate: tw.URL + "/validate", Helix: tw.URL + "/helix", EventSub: ws}
		o.OAuth = twitch.NewOAuth(tw.Client(), o.Endpoints, time.Now)
		o.Dialer = twitch.CoderDialer{HTTP: &http.Client{}}
	}, configureOBS: func(o *obs.Options) {
		o.PollInterval = time.Second
		if os.Getenv("CUEMENDER_P04_BROWSER_DIR") != "" {
			o.PollInterval = time.Second
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
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
		req.Header.Set("X-CSRF-Token", csrf)
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		out, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("%s=%d want%d: %s", path, res.StatusCode, want, out)
		}
		return out
	}
	var bootstrap struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		}
	}
	_ = json.Unmarshal(post("/api/v1/session/bootstrap", map[string]any{"token": a.ui.BootstrapToken()}, "", 200), &bootstrap)
	csrf := bootstrap.Data.CSRF
	post("/api/v1/settings", map[string]any{"expected_revision": 0, "microphone": domain.InputRef{Name: "P04 private mic canary", UUID: "mic-uuid", Context: "Collection"}, "stream_track": 2, "visual": nil}, csrf, 200)
	post("/api/v1/obs/connect", map[string]any{"address": "ws" + strings.TrimPrefix(obsServer.URL, "http"), "password": ""}, csrf, 202)
	waitStatus(t, func() string { return a.obsMgr.Snapshot().Status }, domain.ConnectionConnected)
	post("/api/v1/twitch/connect", map[string]any{"client_id": "abc123"}, csrf, 202)
	waitStatus(t, func() string { return a.twitchMgr.Snapshot().Status }, domain.ConnectionConnected)
	chat.help <- "go"
	select {
	case <-chat.replies:
	case <-time.After(3 * time.Second):
		t.Fatal("local Twitch help not delivered")
	}
	post("/api/v1/monitoring/start", map[string]any{"expected_revision": 1}, csrf, 200)
	chat.chat <- "go"
	var p domain.Proposal
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, e := client.Get(a.BaseURL() + "/api/v1/state")
		if e != nil {
			t.Fatal(e)
		}
		var wrapper struct{ Data domain.PrivateState }
		_ = json.NewDecoder(res.Body).Decode(&wrapper)
		res.Body.Close()
		if len(wrapper.Data.Proposals) > 0 {
			p = wrapper.Data.Proposals[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if p.ID == "" {
		t.Fatal("EventSub→fresh OBS→durable proposal missing")
	}

	if dir := os.Getenv("CUEMENDER_P04_BROWSER_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		origin, _ := url.Parse(a.BaseURL())
		cookies := []map[string]any{}
		for _, cookie := range client.Jar.Cookies(origin) {
			cookies = append(cookies, map[string]any{"name": cookie.Name, "value": cookie.Value, "domain": origin.Hostname(), "path": "/", "httpOnly": true, "secure": false, "sameSite": "Strict", "expires": -1})
		}
		storage, _ := json.Marshal(map[string]any{"cookies": cookies, "origins": []any{}})
		metadata, _ := json.Marshal(map[string]any{"baseURL": a.BaseURL()})
		if err := os.WriteFile(filepath.Join(dir, "storage-state.json"), storage, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "metadata.json"), metadata, 0600); err != nil {
			t.Fatal(err)
		}
		until := time.Now().Add(120 * time.Second)
		stopped := false
		for time.Now().Before(until) {
			if _, err := os.Stat(filepath.Join(dir, "stop")); err == nil {
				stopped = true
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !stopped {
			t.Fatal("browser fixture timed out waiting for stop")
		}
		_, as := a.coordinator.RepairSnapshot()
		fixture.mu.Lock()
		writes := fixture.writes
		fixture.mu.Unlock()
		if len(as) != 1 || as[0].Status != domain.TrackRepairVerified || writes != 1 {
			t.Fatalf("browser did not approve one verified action: actions=%d writes=%d", len(as), writes)
		}
		inc, _ := a.coordinator.Find(p.IncidentID)
		if len(inc.Findings) != 2 || inc.LocalVerification != domain.LocalVerified {
			t.Fatal("browser flow lost remaining diagnosis")
		}
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(4 * time.Second):
			t.Fatal("browser app did not join")
		}
		return
	}
	path := "/api/v1/proposals/" + p.ID + "/approve"
	post(path, map[string]any{"expected_revision": 1}, "", 403)
	post(path, map[string]any{"expected_revision": 1, "input_name": "arbitrary"}, csrf, 400)
	post(path, map[string]any{}, csrf, 400)
	post(path, map[string]any{"expected_revision": 2}, csrf, 409)
	post("/api/v1/proposals/unknown/approve", map[string]any{"expected_revision": 1}, csrf, 404)
	var accepted struct{ Data domain.Action }
	_ = json.Unmarshal(post(path, map[string]any{"expected_revision": 1}, csrf, 202), &accepted)
	deadline = time.Now().Add(5 * time.Second)
	var action domain.Action
	for time.Now().Before(deadline) {
		_, as := a.coordinator.RepairSnapshot()
		for _, got := range as {
			if got.ID == accepted.Data.ID {
				action = got
			}
		}
		if action.Status == domain.TrackRepairVerified {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if action.Status != domain.TrackRepairVerified || action.IntentAt == nil {
		t.Fatalf("action=%+v", action)
	}
	var repeated struct{ Data domain.Action }
	_ = json.Unmarshal(post(path, map[string]any{"expected_revision": 1}, csrf, 202), &repeated)
	if repeated.Data.ID != action.ID {
		t.Fatal("duplicate approval created action")
	}
	fixture.mu.Lock()
	writes, bad := fixture.writes, fixture.bad
	tracks := fixture.tracks
	fixture.mu.Unlock()
	if writes != 1 || bad || !reflect.DeepEqual(tracks, map[string]bool{"1": true, "2": true, "3": false, "4": true, "5": false, "6": true}) {
		t.Fatalf("writes=%d bad=%v map=%v", writes, bad, tracks)
	}
	_, stored, e := a.store.LoadRepairs(ctx)
	if e != nil || len(stored) != 1 || stored[0].Status != domain.TrackRepairVerified {
		t.Fatalf("durable result=%+v err=%v", stored, e)
	}
	inc, _ := a.coordinator.Find(p.IncidentID)
	if len(inc.Findings) != 2 || inc.LocalVerification != domain.LocalVerified || inc.Feedback.Positive != 0 {
		t.Fatalf("remaining mute/local/viewer separation=%+v", inc)
	}
	for _, path := range []string{"/public/v1/state", "/public/v1/events"} {
		req, _ := http.NewRequest("GET", a.BaseURL()+path, nil)
		publicCtx, stop := context.WithTimeout(ctx, time.Second)
		req = req.WithContext(publicCtx)
		res, e := client.Do(req)
		if e != nil {
			stop()
			t.Fatal(e)
		}
		var raw []byte
		if strings.HasSuffix(path, "events") {
			buf := make([]byte, 8192)
			n, _ := res.Body.Read(buf)
			raw = buf[:n]
		} else {
			raw, _ = io.ReadAll(res.Body)
		}
		res.Body.Close()
		stop()
		for _, canary := range []string{"P04 private mic canary", p.ID, action.ID, "expected_tracks", "inputUuid", "access-token", "csrf_token"} {
			if strings.Contains(string(raw), canary) {
				t.Fatalf("public %s leaked %s", path, canary)
			}
		}
	}
	cancel()
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("app did not join")
	}
	a.Close()
}
