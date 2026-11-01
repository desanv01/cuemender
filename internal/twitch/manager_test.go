package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/secrets"
)

type fakeSocket struct {
	ch     chan []byte
	closed chan struct{}
	once   sync.Once
}

func newFakeSocket(frames ...[]byte) *fakeSocket {
	s := &fakeSocket{ch: make(chan []byte, len(frames)+1), closed: make(chan struct{})}
	for _, frame := range frames {
		s.ch <- frame
	}
	return s
}

func (s *fakeSocket) Read(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.closed:
		return nil, errors.New("socket closed")
	case frame := <-s.ch:
		return frame, nil
	}
}

func (s *fakeSocket) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

type fakeDialer struct {
	mu      sync.Mutex
	sockets []*fakeSocket
	calls   int
}

func (d *fakeDialer) Dial(context.Context, string, http.Header) (Socket, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	if len(d.sockets) == 0 {
		return nil, errors.New("no fake socket")
	}
	socket := d.sockets[0]
	d.sockets = d.sockets[1:]
	return socket, nil
}

func testTwitchOAuthServer(t *testing.T, tokenMode string) (*OAuth, *int32) {
	t.Helper()
	var mu sync.Mutex
	var subscribeCalls int32
	var refreshCalls, sendCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "device-secret", "user_code": "ABCD-1234",
				"verification_uri": "https://www.twitch.tv/activate",
				"expires_in":       1800, "interval": 1,
			})
		case "/token":
			_ = r.ParseForm()
			if tokenMode == "revoked" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.Form.Get("grant_type") == "refresh_token" {
				mu.Lock()
				refreshCalls++
				mu.Unlock()
				_ = json.NewEncoder(w).Encode(map[string]any{
					"access_token": "refreshed-access", "refresh_token": "rotated-refresh",
					"expires_in": 3600, "scope": []string{ScopeReadChat, ScopeWriteChat},
				})
				return
			}
			if tokenMode == "pending-block" {
				<-r.Context().Done()
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-token", "refresh_token": "refresh-token",
				"expires_in": 3600, "scope": []string{ScopeReadChat, ScopeWriteChat},
			})
		case "/validate":
			if tokenMode == "revoked" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			clientID := "abc123"
			if tokenMode == "bad-bind" {
				clientID = "other-client"
			}
			scopes := []string{ScopeReadChat, ScopeWriteChat}
			if tokenMode == "missing-scope" {
				scopes = []string{ScopeReadChat}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"client_id": clientID, "login": "broadcaster", "user_id": "user-1",
				"scopes": scopes, "expires_in": 3600,
			})
		case "/helix/eventsub/subscriptions":
			atomic.AddInt32(&subscribeCalls, 1)
			w.WriteHeader(http.StatusAccepted)
		case "/helix/chat/messages":
			mu.Lock()
			sendCalls++
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"is_sent": true}}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	oauth := NewOAuth(server.Client(), testEndpoints(server.URL), time.Now)
	oauth.Sleep = func(context.Context, time.Duration) error { return nil }
	_ = sendCalls
	return oauth, &subscribeCalls
}

func welcomeFrame(sessionID string) []byte {
	return []byte(`{"metadata":{"message_id":"welcome-1","message_type":"session_welcome","message_timestamp":"2026-11-01T12:00:00Z"},"payload":{"session":{"id":"` + sessionID + `","status":"connected","keepalive_timeout_seconds":10}}}`)
}

func chatFrame(deliveryID, messageID, text, broadcaster, chatter string) []byte {
	return []byte(`{"metadata":{"message_id":"` + deliveryID + `","message_type":"notification","message_timestamp":"2026-11-01T12:00:00Z"},"payload":{"subscription":{"type":"channel.chat.message","version":"1"},"event":{"message_id":"` + messageID + `","broadcaster_user_id":"` + broadcaster + `","chatter_user_id":"` + chatter + `","chatter_user_login":"viewer","message_type":"text","message":{"text":"` + text + `"}}}}`)
}

func waitManager(t *testing.T, m *Manager, status string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if m.Snapshot().Status == status {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("manager status = %s, want %s", m.Snapshot().Status, status)
}

func TestManagerDeviceFlowConnectChatAndDisconnect(t *testing.T) {
	oauth, _ := testTwitchOAuthServer(t, "success")
	socket := newFakeSocket(
		welcomeFrame("session-1"),
		chatFrame("delivery-1", "message-1", "!issueaudio", "user-1", "viewer-2"),
	)
	dialer := &fakeDialer{sockets: []*fakeSocket{socket}}
	secretStore := secrets.NewMemory()
	manager := NewManager(Options{
		Context:   context.Background(),
		OAuth:     oauth,
		Store:     NewCredentialStore(secretStore),
		Dialer:    dialer,
		Endpoints: oauth.Endpoints,
		ClientID:  "abc123",
		OnUpdate:  func() {},
	})
	t.Cleanup(manager.Close)
	if err := manager.Connect("abc123"); err != nil {
		t.Fatal(err)
	}
	waitManager(t, manager, domain.ConnectionConnected)
	deadline := time.Now().Add(2 * time.Second)
	for {
		snapshot := manager.Snapshot()
		if snapshot.Receipt.Accepted >= 1 {
			if snapshot.Receipt.LastCommand != string(CommandIssueAudio) || snapshot.Receipt.LastCategory != "audio" {
				t.Fatalf("receipt = %+v", snapshot.Receipt)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("chat notification was not accepted")
		}
		time.Sleep(5 * time.Millisecond)
	}
	manager.mu.Lock()
	replyStarted := manager.reply != nil
	manager.mu.Unlock()
	if !replyStarted {
		t.Fatal("chat command did not start the bounded reply queue")
	}
	if err := manager.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if manager.Snapshot().Status != domain.ConnectionNotConnected {
		t.Fatalf("status after disconnect = %s", manager.Snapshot().Status)
	}
	if _, err := secretStore.Get(context.Background(), credentialKey); !secrets.IsNotFound(err) {
		t.Fatalf("credential was not deleted on disconnect: %v", err)
	}
}

func TestManagerDisconnectCancelsPendingDeviceFlow(t *testing.T) {
	oauth, _ := testTwitchOAuthServer(t, "pending-block")
	manager := NewManager(Options{
		Context:   context.Background(),
		OAuth:     oauth,
		Store:     NewCredentialStore(secrets.NewMemory()),
		Dialer:    &fakeDialer{},
		Endpoints: oauth.Endpoints,
		ClientID:  "abc123",
		OnUpdate:  func() {},
	})
	t.Cleanup(manager.Close)
	if err := manager.Connect("abc123"); err != nil {
		t.Fatal(err)
	}
	waitManager(t, manager, domain.ConnectionAuthorizing)
	if err := manager.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if manager.Snapshot().Status != domain.ConnectionNotConnected {
		t.Fatalf("pending flow revived after disconnect: %s", manager.Snapshot().Status)
	}
}

func TestManagerStartSavedUsesValidatedCredentials(t *testing.T) {
	oauth, _ := testTwitchOAuthServer(t, "success")
	socket := newFakeSocket(welcomeFrame("saved-session"))
	secretStore := secrets.NewMemory()
	credentialStore := NewCredentialStore(secretStore)
	creds := Credentials{
		AccessToken: "saved-access", RefreshToken: "saved-refresh",
		ClientID: "abc123", UserID: "user-1", Login: "broadcaster",
		Scopes: []string{ScopeReadChat, ScopeWriteChat}, ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := credentialStore.Save(context.Background(), creds); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(Options{
		Context:   context.Background(),
		OAuth:     oauth,
		Store:     credentialStore,
		Dialer:    &fakeDialer{sockets: []*fakeSocket{socket}},
		Endpoints: oauth.Endpoints,
		ClientID:  "abc123",
		OnUpdate:  func() {},
	})
	t.Cleanup(manager.Close)
	manager.StartSaved()
	waitManager(t, manager, domain.ConnectionConnected)
	if manager.Snapshot().Login != "broadcaster" {
		t.Fatalf("login = %q", manager.Snapshot().Login)
	}
}

func TestManagerRefreshRotationAndDisconnectRace(t *testing.T) {
	oauth, _ := testTwitchOAuthServer(t, "success")
	secretStore := secrets.NewMemory()
	manager := NewManager(Options{
		Context:   context.Background(),
		OAuth:     oauth,
		Store:     NewCredentialStore(secretStore),
		Dialer:    &fakeDialer{},
		Endpoints: oauth.Endpoints,
		ClientID:  "abc123",
		OnUpdate:  func() {},
	})
	t.Cleanup(manager.Close)
	creds := Credentials{
		AccessToken: "old", RefreshToken: "old-refresh", ClientID: "abc123", UserID: "user-1",
		Scopes: []string{ScopeReadChat, ScopeWriteChat}, ExpiresAt: time.Now().Add(time.Hour),
	}
	manager.mu.Lock()
	manager.creds = &creds
	manager.conn.Status = domain.ConnectionConnected
	manager.mu.Unlock()
	if err := manager.RefreshNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewCredentialStore(secretStore).Load(context.Background())
	if err != nil || loaded.AccessToken != "refreshed-access" || loaded.RefreshToken != "rotated-refresh" {
		t.Fatalf("rotated credentials = %+v, %v", loaded, err)
	}
	if err := manager.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.RefreshNow(context.Background()); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("refresh after disconnect = %v", err)
	}
}

func TestManagerRequestedReconnectHandoverAndDuplicateDedup(t *testing.T) {
	oauth, subscribeCalls := testTwitchOAuthServer(t, "success")
	first := newFakeSocket(
		welcomeFrame("session-1"),
		[]byte(`{"metadata":{"message_id":"reconnect-1","message_type":"session_reconnect","message_timestamp":"2026-11-01T12:00:00Z"},"payload":{"session":{"id":"session-2","status":"reconnecting","reconnect_url":"wss://eventsub.wss.twitch.tv/ws?session_id=2"}}}`),
	)
	second := newFakeSocket(
		welcomeFrame("session-2"),
		chatFrame("delivery-2", "message-2", "!issuevideo", "user-1", "viewer-2"),
		chatFrame("delivery-2", "message-2", "!issuevideo", "user-1", "viewer-2"),
	)
	dialer := &fakeDialer{sockets: []*fakeSocket{second}}
	manager := NewManager(Options{
		Context:   context.Background(),
		OAuth:     oauth,
		Store:     NewCredentialStore(secrets.NewMemory()),
		Dialer:    dialer,
		Endpoints: oauth.Endpoints,
		ClientID:  "abc123",
		OnUpdate:  func() {},
	})
	t.Cleanup(manager.Close)
	creds := Credentials{AccessToken: "access-token", RefreshToken: "refresh-token", ClientID: "abc123", UserID: "user-1", Scopes: []string{ScopeReadChat, ScopeWriteChat}}
	manager.mu.Lock()
	manager.creds = &creds
	manager.gen = 1
	manager.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- manager.runSocket(ctx, 1, first, creds, true) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		select {
		case err := <-errCh:
			t.Fatalf("runSocket returned early: %v", err)
		default:
		}
		snapshot := manager.Snapshot()
		if snapshot.Receipt.Accepted == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("duplicate chat handling did not finish: %+v", snapshot.Receipt)
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-errCh:
	case <-time.After(time.Second):
		t.Fatal("runSocket did not stop")
	}
	if atomic.LoadInt32(subscribeCalls) != 1 {
		t.Fatalf("requested reconnect resubscribed %d times, want 1", atomic.LoadInt32(subscribeCalls))
	}
	if dialer.calls != 1 {
		t.Fatalf("dialer calls = %d, want 1 reconnect dial", dialer.calls)
	}
}

type failingSecrets struct{}

func (failingSecrets) Get(context.Context, string) (string, error) {
	return "", &secrets.Error{Kind: secrets.KindNotFound, Op: "get", Key: "k"}
}
func (failingSecrets) Set(context.Context, string, string) error {
	return &secrets.Error{Kind: secrets.KindUnavailable, Op: "set", Key: "k"}
}
func (failingSecrets) Delete(context.Context, string) error { return nil }

type deleteFailSecrets struct{}

func (deleteFailSecrets) Get(context.Context, string) (string, error) {
	return "", &secrets.Error{Kind: secrets.KindNotFound, Op: "get", Key: "k"}
}
func (deleteFailSecrets) Set(context.Context, string, string) error { return nil }
func (deleteFailSecrets) Delete(context.Context, string) error {
	return &secrets.Error{Kind: secrets.KindUnavailable, Op: "delete", Key: "k"}
}

func TestManagerStartSavedRevocationNeedsAttention(t *testing.T) {
	oauth, _ := testTwitchOAuthServer(t, "revoked")
	secretStore := secrets.NewMemory()
	credentialStore := NewCredentialStore(secretStore)
	creds := Credentials{
		AccessToken: "revoked-access", RefreshToken: "revoked-refresh",
		ClientID: "abc123", UserID: "user-1", Login: "broadcaster",
		Scopes: []string{ScopeReadChat, ScopeWriteChat}, ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := credentialStore.Save(context.Background(), creds); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(Options{
		Context:   context.Background(),
		OAuth:     oauth,
		Store:     credentialStore,
		Dialer:    &fakeDialer{},
		Endpoints: oauth.Endpoints,
		ClientID:  "abc123",
		OnUpdate:  func() {},
	})
	t.Cleanup(manager.Close)
	manager.StartSaved()
	waitManager(t, manager, domain.ConnectionNeedsAttention)
	if manager.Snapshot().DetailCode != "authorization_required" {
		t.Fatalf("revocation detail = %+v", manager.Snapshot())
	}
}

func TestManagerDisconnectDeleteFailureIsExplicit(t *testing.T) {
	manager := NewManager(Options{
		Context:  context.Background(),
		Store:    NewCredentialStore(deleteFailSecrets{}),
		Dialer:   &fakeDialer{},
		ClientID: "abc123",
		OnUpdate: func() {},
	})
	t.Cleanup(manager.Close)
	creds := Credentials{AccessToken: "a", RefreshToken: "r", ClientID: "abc123", UserID: "user-1"}
	manager.mu.Lock()
	manager.creds = &creds
	manager.conn.Status = domain.ConnectionConnected
	manager.mu.Unlock()
	err := manager.Disconnect(context.Background())
	if !errors.Is(err, ErrCredentialDeleteFailed) {
		t.Fatalf("disconnect error = %v, want delete failure", err)
	}
	if manager.Snapshot().Status != domain.ConnectionNeedsAttention {
		t.Fatalf("delete failure status = %s", manager.Snapshot().Status)
	}
}

func TestManagerRejectsTokenBindingMismatch(t *testing.T) {
	oauth, _ := testTwitchOAuthServer(t, "bad-bind")
	manager := NewManager(Options{
		Context:   context.Background(),
		OAuth:     oauth,
		Store:     NewCredentialStore(secrets.NewMemory()),
		Dialer:    &fakeDialer{},
		Endpoints: oauth.Endpoints,
		ClientID:  "abc123",
		OnUpdate:  func() {},
	})
	t.Cleanup(manager.Close)
	if err := manager.Connect("abc123"); err != nil {
		t.Fatal(err)
	}
	waitManager(t, manager, domain.ConnectionNeedsAttention)
	if manager.Snapshot().DetailCode != "token_binding_mismatch" {
		t.Fatalf("detail = %+v", manager.Snapshot())
	}
}

func TestManagerCredentialSaveFailureIsExplicit(t *testing.T) {
	oauth, _ := testTwitchOAuthServer(t, "success")
	socket := newFakeSocket(welcomeFrame("save-failure-session"))
	manager := NewManager(Options{
		Context:   context.Background(),
		OAuth:     oauth,
		Store:     NewCredentialStore(failingSecrets{}),
		Dialer:    &fakeDialer{sockets: []*fakeSocket{socket}},
		Endpoints: oauth.Endpoints,
		ClientID:  "abc123",
		OnUpdate:  func() {},
	})
	t.Cleanup(manager.Close)
	if err := manager.Connect("abc123"); err != nil {
		t.Fatal(err)
	}
	waitManager(t, manager, domain.ConnectionConnected)
	if !manager.Snapshot().CredentialPersistenceDegraded {
		t.Fatalf("save failure not exposed: %+v", manager.Snapshot())
	}
}

func TestManagerOldGenerationStateIgnored(t *testing.T) {
	manager := NewManager(Options{Context: context.Background(), OnUpdate: func() {}})
	manager.mu.Lock()
	manager.gen = 5
	manager.mu.Unlock()
	manager.setState(4, domain.ConnectionConnected, "connected", "stale")
	if manager.Snapshot().Status == domain.ConnectionConnected {
		t.Fatal("old generation state was installed")
	}
}

func TestReadEnvelopeHonorsDeadline(t *testing.T) {
	socket := newFakeSocket()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	start := time.Now()
	_, err := readEnvelope(ctx, socket, 20*time.Millisecond)
	if err == nil {
		t.Fatal("blocked socket read did not time out")
	}
	if time.Since(start) > time.Second {
		t.Fatal("read deadline was not bounded")
	}
}

type oneFrameSocket struct {
	frame []byte
	read  bool
}

func (s *oneFrameSocket) Read(context.Context) ([]byte, error) {
	if s.read {
		return nil, errors.New("simulated ordinary socket loss")
	}
	s.read = true
	return s.frame, nil
}
func (s *oneFrameSocket) Close() error { return nil }

type sequenceDialer struct {
	mu      sync.Mutex
	sockets []Socket
	calls   int
}

func (d *sequenceDialer) Dial(context.Context, string, http.Header) (Socket, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	if len(d.sockets) == 0 {
		return nil, errors.New("no scripted socket")
	}
	socket := d.sockets[0]
	d.sockets = d.sockets[1:]
	return socket, nil
}

func TestEventSubOrdinaryReconnectResubscribes(t *testing.T) {
	oauth, subscribeCalls := testTwitchOAuthServer(t, "success")
	first := &oneFrameSocket{frame: welcomeFrame("old-session")}
	good := newFakeSocket(welcomeFrame("new-session"))
	dialer := &sequenceDialer{sockets: []Socket{first, good}}
	manager := NewManager(Options{
		Context:   context.Background(),
		OAuth:     oauth,
		Store:     NewCredentialStore(secrets.NewMemory()),
		Dialer:    dialer,
		Endpoints: oauth.Endpoints,
		ClientID:  "abc123",
		OnUpdate:  func() {},
	})
	t.Cleanup(manager.Close)
	creds := Credentials{AccessToken: "access-token", RefreshToken: "refresh-token", ClientID: "abc123", UserID: "user-1", Scopes: []string{ScopeReadChat, ScopeWriteChat}}
	manager.mu.Lock()
	manager.creds = &creds
	manager.gen = 1
	manager.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	manager.startEventSub(ctx, 1)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(subscribeCalls) >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if atomic.LoadInt32(subscribeCalls) < 2 {
		t.Fatalf("ordinary reconnect did not resubscribe: %d status=%+v dials=%d", atomic.LoadInt32(subscribeCalls), manager.Snapshot(), dialer.calls)
	}
	if dialer.calls < 2 {
		t.Fatalf("dialer calls = %d", dialer.calls)
	}
}
