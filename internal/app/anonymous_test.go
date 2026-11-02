package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/twitch"
)

func TestReceiveOnlyActualAppHTTPAndProtocol(t *testing.T) {
	p03ObsMu.Lock()
	p03ObsRequests = nil
	p03ObsMu.Unlock()
	obsServer := localOBSServer(t)
	peers := make(chan net.Conn, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := t.TempDir()
	a, err := newWithDependencies(ctx, Options{ReceiveOnlyChannel: "testchannel", DataDir: base, ListenAddress: freeP05Address(t), NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard}, appDependencies{configureAnonymous: func(o *twitch.AnonymousOptions) {
		o.Dial = func(ctx context.Context) (net.Conn, error) { c, s := net.Pipe(); peers <- s; return c, nil }
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.twitchMgr != nil || a.DataDir() != filepath.Join(base, "receive-only", "testchannel") {
		t.Fatal("composition not isolated")
	}
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	waitHealthy(t, a.BaseURL())
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
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
			t.Fatalf("%s=%d want %d %s", path, res.StatusCode, want, out)
		}
		return out
	}
	endpoint := "/api/v1/twitch/receive-only/connect"
	post(endpoint, map[string]any{"channel": "testchannel", "confirmed": true}, "", 401)
	var boot struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		}
	}
	json.Unmarshal(post("/api/v1/session/bootstrap", map[string]any{"token": a.ui.BootstrapToken()}, "", 200), &boot)
	csrf := boot.Data.CSRF
	post(endpoint, map[string]any{"channel": "testchannel", "confirmed": true}, "", 403)
	post(endpoint, map[string]any{"channel": "other", "confirmed": true}, csrf, 400)
	post(endpoint, map[string]any{"channel": "testchannel", "confirmed": false}, csrf, 400)
	post("/api/v1/twitch/connect", map[string]any{"client_id": "abc123"}, csrf, 409)
	post(endpoint, map[string]any{"channel": "testchannel", "confirmed": true}, csrf, 202)
	var peer net.Conn
	select {
	case peer = <-peers:
	case <-time.After(time.Second):
		t.Fatal("no IRC dial")
	}
	defer peer.Close()
	reader := bufio.NewReader(peer)
	for i := 0; i < 4; i++ {
		peer.SetReadDeadline(time.Now().Add(time.Second))
		if _, e := reader.ReadString('\n'); e != nil {
			t.Fatal(e)
		}
	}
	fmt.Fprint(peer, ":tmi.twitch.tv CAP * ACK :twitch.tv/tags twitch.tv/commands\r\n@room-id=123 :tmi.twitch.tv ROOMSTATE #testchannel\r\n:tmi.twitch.tv 366 justinfan123123 #testchannel :End\r\n")
	waitStatus(t, func() string { return a.chatSnapshot().Status }, domain.ConnectionConnected)
	a.stateMu.Lock()
	projection := a.privateLocked()
	public := a.publicLocked()
	a.stateMu.Unlock()
	if projection.Setup.TwitchAuthorized || !projection.Setup.ChatConnected || !projection.Twitch.ReceiveOnly || public.ChatMode != "receive_only" {
		t.Fatal("misrepresented capability")
	}
	if err := a.TwitchConnect(ctx, "abc123"); !errors.Is(err, domain.ErrModeConflict) {
		t.Fatal("OAuth reachable")
	}
	track := 2
	settings, err := a.SaveSettings(ctx, 0, domain.Expectations{Microphone: &domain.InputRef{Name: "Test Mic", UUID: "mic-uuid", Context: "Collection"}, StreamTrack: &track})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.OBSConnect(ctx, "ws"+strings.TrimPrefix(obsServer.URL, "http"), nil); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, func() string { return a.obsMgr.Snapshot().Status }, domain.ConnectionConnected)
	if _, err := a.StartMonitoring(ctx, settings.Revision); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(peer, "@room-id=123;user-id=456;id=message-123456;tmi-sent-ts=%d :viewer!viewer@viewer.tmi.twitch.tv PRIVMSG #testchannel :!issueaudio\r\n", time.Now().UnixMilli())
	deadline := time.Now().Add(5 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		list, e := a.store.ListIncidents(ctx, 10)
		if e != nil {
			t.Fatal(e)
		}
		if len(list) > 0 && len(list[0].Findings) == 2 {
			if list[0].RepairAvailable || list[0].Stage == domain.StageAwaitingApproval {
				t.Fatal("repair offered")
			}
			found = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !found {
		t.Fatal("IRC to real App/OBS protocol/SQLite diagnostic missing")
	}
	proposals, actions := a.coordinator.RepairSnapshot()
	if len(proposals) > 0 || len(actions) > 0 {
		t.Fatal("repair records created")
	}
	if _, err := a.ApproveProposal(ctx, "forged", settings.Revision); !errors.Is(err, domain.ErrModeConflict) {
		t.Fatal("approval reachable")
	}
	// No IRC bot response exists even when the coordinator generated an acknowledgment.
	peer.SetReadDeadline(time.Now().Add(30 * time.Millisecond))
	if line, e := reader.ReadString('\n'); e == nil {
		t.Fatalf("unexpected IRC outbound %q", line)
	}
	if err := a.TwitchDisconnect(ctx); err != nil {
		t.Fatal(err)
	}
	if a.chatSnapshot().Status != domain.ConnectionNotConnected {
		t.Fatal("not disconnected")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("app close did not join")
	}
	p03ObsMu.Lock()
	requests := append([]string(nil), p03ObsRequests...)
	p03ObsMu.Unlock()
	allowed := map[string]bool{"GetVersion": true, "GetInputList": true, "GetSceneList": true, "GetSceneItemList": true, "GetSceneCollectionList": true, "GetCurrentProgramScene": true, "GetInputMute": true, "GetInputAudioTracks": true, "GetSceneItemEnabled": true, "GetStreamStatus": true}
	for _, r := range requests {
		if !allowed[r] {
			t.Fatalf("OBS write/unexpected request: %s", r)
		}
	}
}

func TestReceiveOnlyRejectsRehearsalAndInvalidChannel(t *testing.T) {
	for _, o := range []Options{{Mode: domain.ModeRehearsal, ReceiveOnlyChannel: "testchannel"}, {ReceiveOnlyChannel: "../bad"}, {ReceiveOnlyChannel: "UPPER"}} {
		o.DataDir = t.TempDir()
		if a, err := New(context.Background(), o); err == nil {
			a.Close()
			t.Fatal("invalid receive-only options accepted")
		}
	}
}
