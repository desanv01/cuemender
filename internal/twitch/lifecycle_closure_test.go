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

func awaitClosed(t *testing.T, done <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not finish", label)
	}
}

func seedAuthorization(m *Manager) Credentials {
	creds := reviewCredentials()
	m.mu.Lock()
	m.creds = &creds
	m.gen = 1
	m.conn.Status = domain.ConnectionConnected
	m.mu.Unlock()
	return creds
}

func TestStartupCredentialAndValidationBoundaries(t *testing.T) {
	for _, scenario := range []string{"valid", "missing", "malformed", "incomplete", "changed_subject", "rotated_subject", "expired_rotation", "second_validation_transient", "first_validation_transient", "refresh_transient"} {
		t.Run(scenario, func(t *testing.T) {
			var validates, refreshes, unexpected atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/validate":
					n := validates.Add(1)
					if (scenario == "second_validation_transient" && n == 2) || (scenario == "first_validation_transient" && n == 1) {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					if n == 1 && (scenario == "rotated_subject" || scenario == "expired_rotation" || scenario == "refresh_transient") {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					user := "user-1"
					if scenario == "changed_subject" || scenario == "rotated_subject" {
						user = "user-2"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "abc123", "user_id": user, "login": "broadcaster", "scopes": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": 3600})
				case "/token":
					refreshes.Add(1)
					if scenario == "refresh_transient" {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					expires := 3600
					if scenario == "expired_rotation" {
						expires = 0
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "rotated", "refresh_token": "rotated-refresh", "scope": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": expires})
				case "/helix/eventsub/subscriptions":
					w.WriteHeader(http.StatusAccepted)
				default:
					unexpected.Add(1)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			memory := secrets.NewMemory()
			store := NewCredentialStore(memory)
			if scenario != "missing" {
				if err := store.Save(context.Background(), reviewCredentials()); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "malformed" || scenario == "incomplete" {
				value := "{"
				if scenario == "incomplete" {
					value = "{}"
				}
				if err := memory.Set(context.Background(), credentialKey, value); err != nil {
					t.Fatal(err)
				}
			}
			oauth := NewOAuth(server.Client(), testEndpoints(server.URL), time.Now)
			dialer := &fakeDialer{sockets: []*fakeSocket{newFakeSocket(welcomeFrame("startup"))}}
			m := NewManager(Options{OAuth: oauth, Store: store, ClientID: "abc123", Dialer: dialer, Endpoints: oauth.Endpoints, RetryWait: func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil }})
			defer m.Close()
			m.StartSaved()
			want := domain.ConnectionNeedsAttention
			if scenario == "valid" || scenario == "second_validation_transient" || scenario == "first_validation_transient" {
				want = domain.ConnectionConnected
			}
			if scenario == "missing" {
				want = domain.ConnectionNotConnected
			}
			waitManager(t, m, want)
			if want != domain.ConnectionConnected {
				m.workerWG.Wait()
				if m.currentCredentials() != nil || dialer.calls != 0 {
					t.Fatal("unverified startup credential reached chat transport")
				}
			}
			if (scenario == "missing" || scenario == "malformed" || scenario == "incomplete") && validates.Load() != 0 {
				t.Fatal("missing/invalid stored credential was sent to provider")
			}
			if scenario == "refresh_transient" && m.Snapshot().DetailCode != "authorization_unavailable" {
				t.Fatal("transient refresh failure was called revocation")
			}
			if scenario == "second_validation_transient" && (validates.Load() != 3 || refreshes.Load() != 0) {
				t.Fatalf("startup validation retries=%d, refreshes=%d", validates.Load(), refreshes.Load())
			}
			if unexpected.Load() != 0 || m.Snapshot().Device != nil {
				t.Fatal("startup spontaneously began device authorization")
			}
		})
	}
}

func TestStartupUnavailableCredentialStorageIsExplicit(t *testing.T) {
	m := NewManager(Options{ClientID: "abc123", StorageMode: domain.SecretsModeUnavailable})
	m.StartSaved()
	if snapshot := m.Snapshot(); snapshot.Status != domain.ConnectionNeedsAttention || snapshot.DetailCode != "credential_storage_unavailable" {
		t.Fatalf("unavailable storage = %+v", snapshot)
	}
	m.Close()
	before := m.Snapshot()
	m.StartSaved()
	if after := m.Snapshot(); after.Status != before.Status || after.DetailCode != before.DetailCode {
		t.Fatal("startup mutated an already closed manager")
	}
}

func TestDisconnectCancelsAndJoinsExplicitRefresh(t *testing.T) {
	for _, blocked := range []string{"refresh", "rotated_validation"} {
		t.Run(blocked, func(t *testing.T) {
			entered, requestDone := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				if r.URL.Path == "/token" && blocked == "rotated_validation" {
					_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "next", "refresh_token": "next-refresh", "scope": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": 3600})
					return
				}
				close(entered)
				<-r.Context().Done()
				close(requestDone)
			}))
			defer server.Close()
			oauth := NewOAuth(server.Client(), testEndpoints(server.URL), time.Now)
			m := NewManager(Options{OAuth: oauth, Store: NewCredentialStore(secrets.NewMemory())})
			defer m.Close()
			seedAuthorization(m)
			refreshed := make(chan struct{})
			go func() {
				defer close(refreshed)
				if err := m.RefreshNow(context.Background()); err == nil {
					t.Error("cancelled refresh succeeded")
				}
			}()
			awaitClosed(t, entered, "blocked provider request")
			if err := m.Disconnect(context.Background()); err != nil {
				t.Fatal(err)
			}
			awaitClosed(t, refreshed, "explicit refresh")
			awaitClosed(t, requestDone, "cancelled provider request")
			if m.currentCredentials() != nil || m.Snapshot().Status != domain.ConnectionNotConnected {
				t.Fatal("cancelled refresh revived authorization")
			}
		})
	}
}

func TestStaleCommandAndReplyCannotEnterReplacementAuthorization(t *testing.T) {
	entered, release, commandDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var blockFirst atomic.Bool
	var commands atomic.Int32
	oauth, _ := testTwitchOAuthServer(t, "success")
	m := NewManager(Options{
		OAuth: oauth, Store: NewCredentialStore(secrets.NewMemory()), ClientID: "abc123", Endpoints: oauth.Endpoints,
		Dialer: &fakeDialer{sockets: []*fakeSocket{newFakeSocket(welcomeFrame("replacement"))}},
		Now: func() time.Time {
			if blockFirst.CompareAndSwap(false, true) {
				close(entered)
				<-release
			}
			return time.Now()
		}, OnCommand: func(CommandEvent) { commands.Add(1) },
	})
	defer m.Close()
	seedAuthorization(m)
	m.startReplyQueue(1)
	m.mu.Lock()
	oldReply := m.reply
	m.mu.Unlock()
	event := ChatEvent{MessageID: "old-message", BroadcasterUserID: "user-1", ChatterUserID: "viewer-2"}
	event.Message.Text = "!issueaudio"
	go func() { m.handleChat(1, event, "old-delivery"); close(commandDone) }()
	awaitClosed(t, entered, "old command before admission")
	if err := m.Disconnect(context.Background()); err != nil {
		close(release)
		t.Fatal(err)
	}
	if err := m.Connect("abc123"); err != nil {
		close(release)
		t.Fatal(err)
	}
	waitManager(t, m, domain.ConnectionConnected)
	close(release)
	awaitClosed(t, commandDone, "old command")
	if commands.Load() != 0 || m.Snapshot().Receipt.Accepted != 0 {
		t.Fatal("old command mutated replacement receipt or command stream")
	}
	// Even a delayed invocation of the old sender cannot use new credentials.
	if err := oldReply.sender(context.Background(), replyAck); !errors.Is(err, context.Canceled) {
		t.Fatalf("stale sender = %v", err)
	}
	if oldReply.Enqueue(replyAck) {
		t.Fatal("old queue accepted a replacement lifecycle's work")
	}
}

func TestTerminalReplyFailureCancelsWithoutSelfJoin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer server.Close()
	oauth := NewOAuth(server.Client(), testEndpoints(server.URL), time.Now)
	m := NewManager(Options{OAuth: oauth})
	defer m.Close()
	seedAuthorization(m)
	m.startReplyQueue(1)
	m.mu.Lock()
	queue := m.reply
	m.mu.Unlock()
	queue.Enqueue(replyAck)
	waitManager(t, m, domain.ConnectionNeedsAttention)
	awaitClosed(t, queue.done, "terminal reply sender")
	if m.currentCredentials() != nil || queue.Enqueue(replyAck) {
		t.Fatal("terminal reply failure left authorization intake active")
	}
}

func TestConcurrentLifecycleRegistrationAndClose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = r.ParseForm(); <-r.Context().Done() }))
	defer server.Close()
	oauth := NewOAuth(server.Client(), testEndpoints(server.URL), time.Now)
	for range 20 {
		m := NewManager(Options{OAuth: oauth, Store: NewCredentialStore(secrets.NewMemory()), ClientID: "abc123", RefreshPeriod: time.Millisecond})
		var callers sync.WaitGroup
		for _, action := range []func(){func() { _ = m.Connect("abc123") }, m.StartSaved, m.StartRefreshLoop, func() { _ = m.Disconnect(context.Background()) }, m.Close} {
			callers.Add(1)
			go func() { defer callers.Done(); action() }()
		}
		joined := make(chan struct{})
		go func() { callers.Wait(); close(joined) }()
		awaitClosed(t, joined, "concurrent lifecycle callers")
		if err := m.Connect("abc123"); !errors.Is(err, ErrBusy) {
			t.Fatalf("closed manager accepted Connect: %v", err)
		}
		m.StartRefreshLoop()
		m.workerWG.Wait()
		m.refreshWG.Wait()
	}
}

func TestRotatedValidationTransientDoesNotReplayRefresh(t *testing.T) {
	var refreshes, validations atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			refreshes.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "rotated", "refresh_token": "rotated-refresh", "scope": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": 3600})
			return
		}
		if validations.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "abc123", "user_id": "user-1", "scopes": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": 3600})
	}))
	defer server.Close()
	oauth := NewOAuth(server.Client(), testEndpoints(server.URL), time.Now)
	m := NewManager(Options{OAuth: oauth, Store: NewCredentialStore(secrets.NewMemory()), RetryWait: func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil }})
	defer m.Close()
	seedAuthorization(m)
	if err := m.RefreshNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	if refreshes.Load() != 1 || validations.Load() != 2 || m.currentCredentials().AccessToken != "rotated" || m.Snapshot().Status != domain.ConnectionConnected {
		t.Fatal("transient rotated-token validation lost the new pair or replayed the rotation")
	}
}

func TestLateMaintenanceCannotOverwriteRotatedCredentialMetadata(t *testing.T) {
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "rotated", "refresh_token": "rotated-refresh", "scope": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": 3600})
			return
		}
		expires, login := 3600, "current"
		if r.Header.Get("Authorization") == "OAuth synthetic-access" {
			close(entered)
			<-release
			expires, login = 60, "old"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "abc123", "user_id": "user-1", "login": login, "scopes": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": expires})
	}))
	defer server.Close()
	oauth := NewOAuth(server.Client(), testEndpoints(server.URL), time.Now)
	m := NewManager(Options{OAuth: oauth, Store: NewCredentialStore(secrets.NewMemory())})
	defer m.Close()
	seedAuthorization(m)
	go func() { m.maintenance(); close(done) }()
	awaitClosed(t, entered, "old-token maintenance")
	if err := m.RefreshNow(context.Background()); err != nil {
		close(release)
		t.Fatal(err)
	}
	close(release)
	awaitClosed(t, done, "old-token maintenance completion")
	if creds := m.currentCredentials(); creds.AccessToken != "rotated" || creds.Login != "current" || time.Until(creds.ExpiresAt) < 59*time.Minute {
		t.Fatal("old validation overwrote new credential metadata")
	}
}

func TestLateSocketWelcomeCannotChangeDisconnectedGeneration(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	m := NewManager(Options{Now: func() time.Time { close(entered); <-release; return time.Now() }})
	defer m.Close()
	creds := seedAuthorization(m)
	sock := newFakeSocket(welcomeFrame("late"))
	done := make(chan error, 1)
	go func() { done <- m.runSocket(context.Background(), 1, sock, creds, false) }()
	awaitClosed(t, entered, "welcome before generation installation")
	if err := m.Disconnect(context.Background()); err != nil {
		close(release)
		t.Fatal(err)
	}
	close(release)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("late welcome result=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("late welcome did not exit")
	}
	if snapshot := m.Snapshot(); snapshot.Status != domain.ConnectionNotConnected || snapshot.ConnectionGeneration != 0 {
		t.Fatal("stale socket changed disconnected connection generation")
	}
}

func TestTerminalSubscriptionFailureStopsReconnect(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(status) }))
			defer server.Close()
			oauth := NewOAuth(server.Client(), testEndpoints(server.URL), time.Now)
			dialer := &fakeDialer{sockets: []*fakeSocket{newFakeSocket(welcomeFrame("rejected"))}}
			m := NewManager(Options{OAuth: oauth, Dialer: dialer, Endpoints: oauth.Endpoints})
			defer m.Close()
			seedAuthorization(m)
			m.startEventSub(m.authCtx, 1)
			waitManager(t, m, domain.ConnectionNeedsAttention)
			m.workerWG.Wait()
			if calls.Load() != 1 || dialer.calls != 1 || m.currentCredentials() != nil {
				t.Fatal("terminal Helix failure left reconnect or authorization active")
			}
		})
	}
}
