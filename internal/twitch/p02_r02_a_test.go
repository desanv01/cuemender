package twitch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/secrets"
)

func TestMaintenanceDurationAndShortTokenRefresh(t *testing.T) {
	var expiresIn atomic.Int64
	var refreshes atomic.Int32
	var validates atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/validate":
			validates.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"client_id": "abc123", "user_id": "user-1", "login": "broadcaster",
				"scopes": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": expiresIn.Load(),
			})
		case "/token":
			refreshes.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "short-next", "refresh_token": "short-next-refresh",
				"scope": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": 3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	now := time.Now()
	oauth := NewOAuth(server.Client(), Endpoints{Validate: server.URL + "/validate", Token: server.URL + "/token", Helix: server.URL, EventSub: DefaultEndpoints().EventSub}, func() time.Time { return now })
	manager := NewManager(Options{
		OAuth: oauth, Store: NewCredentialStore(secrets.NewMemory()),
		Now: func() time.Time { return now },
	})
	defer manager.Close()
	creds := Credentials{AccessToken: "old", RefreshToken: "old-refresh", ClientID: "abc123", UserID: "user-1", Scopes: []string{ScopeReadChat, ScopeWriteChat}}
	manager.mu.Lock()
	manager.creds = &creds
	manager.conn.Status = domain.ConnectionConnected
	manager.mu.Unlock()

	expiresIn.Store(60)
	manager.maintenance()
	if refreshes.Load() == 0 {
		t.Fatal("60-second token was not refreshed before expiry")
	}
	if got := manager.currentCredentials(); got == nil || got.AccessToken != "short-next" {
		t.Fatalf("short-token rotated pair not installed: %+v", got)
	}
	if got := manager.currentCredentials().ExpiresAt.Sub(now); got != time.Minute {
		t.Fatalf("expiry duration = %s, want 1m", got)
	}

	expiresIn.Store(3600)
	before := refreshes.Load()
	manager.mu.Lock()
	manager.creds = &Credentials{AccessToken: "hour", RefreshToken: "hour-refresh", ClientID: "abc123", UserID: "user-1", Scopes: []string{ScopeReadChat, ScopeWriteChat}}
	manager.conn.Status = domain.ConnectionConnected
	manager.mu.Unlock()
	manager.maintenance()
	if refreshes.Load() != before {
		t.Fatal("one-hour token was refreshed prematurely")
	}
	if got := manager.currentCredentials().ExpiresAt.Sub(now); got != time.Hour {
		t.Fatalf("one-hour expiry = %s", got)
	}

	expiresIn.Store(0)
	before = refreshes.Load()
	manager.maintenance()
	if manager.Snapshot().Status != domain.ConnectionNeedsAttention {
		t.Fatal("zero/negative expiry did not require authorization")
	}
	if refreshes.Load() != before {
		t.Fatal("invalid zero expiry was treated as refreshable")
	}
}
