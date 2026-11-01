// Promoted from Codex's review-only regression set. The behavioral assertions
// are unchanged; these now run in the ordinary suite.
package twitch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/secrets"
)

func reviewCredentials() Credentials {
	return Credentials{AccessToken: "synthetic-access", RefreshToken: "synthetic-refresh", ClientID: "abc123", UserID: "user-1", Scopes: []string{ScopeReadChat, ScopeWriteChat}, ExpiresAt: time.Now().Add(time.Hour)}
}

func TestReviewHourlyLoopActuallyValidates(t *testing.T) {
	var validated, refreshed atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/validate":
			validated.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "abc123", "user_id": "user-1", "scopes": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": 3600})
		case "/token":
			refreshed.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "synthetic-next", "refresh_token": "synthetic-next-refresh", "scope": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": 3600})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	oauth := NewOAuth(server.Client(), testEndpoints(server.URL), time.Now)
	m := NewManager(Options{OAuth: oauth, Store: NewCredentialStore(secrets.NewMemory()), RefreshPeriod: 10 * time.Millisecond})
	defer m.Close()
	c := reviewCredentials()
	m.mu.Lock()
	m.creds = &c
	m.conn.Status = domain.ConnectionConnected
	m.mu.Unlock()
	m.StartRefreshLoop()
	deadline := time.Now().Add(time.Second)
	for validated.Load() == 0 && refreshed.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if validated.Load() == 0 {
		t.Fatalf("scheduled maintenance performed %d refreshes and zero token validations", refreshed.Load())
	}
}

type reviewBlockingStore struct {
	*secrets.Memory
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *reviewBlockingStore) Set(ctx context.Context, key, value string) error {
	s.once.Do(func() { close(s.entered) })
	<-s.release
	return s.Memory.Set(context.Background(), key, value)
}
func TestReviewRefreshCannotRestoreDeletedCredential(t *testing.T) {
	oauth, _ := testTwitchOAuthServer(t, "success")
	store := &reviewBlockingStore{Memory: secrets.NewMemory(), entered: make(chan struct{}), release: make(chan struct{})}
	m := NewManager(Options{OAuth: oauth, Store: NewCredentialStore(store)})
	defer m.Close()
	c := reviewCredentials()
	m.mu.Lock()
	m.creds = &c
	m.gen = 1
	m.mu.Unlock()
	result := make(chan error, 1)
	go func() { result <- m.RefreshNow(context.Background()) }()
	select {
	case <-store.entered:
	case <-time.After(time.Second):
		close(store.release)
		t.Fatal("refresh did not reach credential persistence")
	}
	disconnected := make(chan error, 1)
	go func() { disconnected <- m.Disconnect(context.Background()) }()
	// A correct implementation can serialize deletion after the pending write.
	// Give it a chance to block; then release persistence and await both operations.
	select {
	case err := <-disconnected:
		if err != nil {
			t.Error(err)
		}
		close(store.release)
		<-result
	case <-time.After(50 * time.Millisecond):
		close(store.release)
		<-result
		if err := <-disconnected; err != nil {
			t.Error(err)
		}
	}
	if _, err := store.Memory.Get(context.Background(), credentialKey); !secrets.IsNotFound(err) {
		t.Fatal("credential persisted after Disconnect returned success")
	}
}

func TestReviewHandoverClosesReplacementSocket(t *testing.T) {
	oauth, _ := testTwitchOAuthServer(t, "success")
	first := newFakeSocket(welcomeFrame("old"), []byte(
		"{\"metadata\":{\"message_id\":\"review-reconnect\",\"message_type\":\"session_reconnect\"},\"payload\":{\"session\":{\"reconnect_url\":\"wss://eventsub.wss.twitch.tv/ws?review=1\"}}}"))
	second := newFakeSocket(welcomeFrame("new"))
	m := NewManager(Options{OAuth: oauth, Dialer: &fakeDialer{sockets: []*fakeSocket{second}}, Endpoints: oauth.Endpoints})
	defer m.Close()
	c := reviewCredentials()
	m.mu.Lock()
	m.creds = &c
	m.gen = 1
	m.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- m.runSocket(ctx, 1, first, c, false) }()
	deadline := time.Now().Add(time.Second)
	for m.Snapshot().Detail != "Twitch chat reconnected." && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if m.Snapshot().Detail != "Twitch chat reconnected." {
		cancel()
		<-done
		t.Fatal("handover fixture did not complete")
	}
	cancel()
	<-done
	_ = first.Close() // startEventSub closes its original socket after runSocket returns.
	select {
	case <-second.closed:
	default:
		t.Error("replacement EventSub socket was not closed on lifecycle cancellation")
	}
	_ = second.Close() // fixture cleanup even when the assertion fails
}
