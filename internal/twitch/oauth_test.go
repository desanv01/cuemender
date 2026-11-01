package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/secrets"
)

func testEndpoints(base string) Endpoints {
	return Endpoints{
		Device:   base + "/device",
		Token:    base + "/token",
		Validate: base + "/validate",
		Helix:    base + "/helix",
		EventSub: "wss://eventsub.wss.twitch.tv/ws",
	}
}

func testOAuth(t *testing.T, handler http.HandlerFunc, now func() time.Time) (*OAuth, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	oauth := NewOAuth(server.Client(), testEndpoints(server.URL), now)
	return oauth, server
}

func TestDeviceFlowPendingSlowDownSuccess(t *testing.T) {
	var mu sync.Mutex
	polls := 0
	var sleeps []time.Duration
	oauth, _ := testOAuth(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device":
			if err := r.ParseForm(); err != nil || r.Form.Get("client_id") != "abc123" {
				t.Errorf("bad device form: %v", r.Form)
			}
			if !strings.Contains(r.Form.Get("scopes"), ScopeReadChat) || !strings.Contains(r.Form.Get("scopes"), ScopeWriteChat) {
				t.Error("device scopes missing chat scopes")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "device-secret", "user_code": "ABCD-1234",
				"verification_uri": "https://www.twitch.tv/activate",
				"expires_in":       1800, "interval": 1,
			})
		case "/token":
			mu.Lock()
			polls++
			current := polls
			mu.Unlock()
			if current == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"message": "authorization_pending"})
				return
			}
			if current == 2 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"message": "slow_down"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-sentinel", "refresh_token": "refresh-sentinel",
				"expires_in": 3600, "scope": []string{ScopeReadChat, ScopeWriteChat},
			})
		default:
			http.NotFound(w, r)
		}
	}, func() time.Time { return time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC) })
	oauth.Sleep = func(_ context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		return nil
	}
	start, err := oauth.StartDevice(context.Background(), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	creds, err := oauth.PollDevice(context.Background(), "abc123", start)
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessToken != "access-sentinel" || creds.RefreshToken != "refresh-sentinel" {
		t.Fatalf("unexpected credentials: %+v", creds)
	}
	if len(sleeps) != 2 || sleeps[0] != time.Second || sleeps[1] != 6*time.Second {
		t.Fatalf("unexpected sleep intervals: %v", sleeps)
	}
}

func TestDeviceFlowDeniedAndExpired(t *testing.T) {
	oauth, _ := testOAuth(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "d", "user_code": "U", "verification_uri": "https://www.twitch.tv/activate",
				"expires_in": 2, "interval": 1,
			})
		case "/token":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "access_denied"})
		default:
			http.NotFound(w, r)
		}
	}, func() time.Time { return time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC) })
	start, err := oauth.StartDevice(context.Background(), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = oauth.PollDevice(context.Background(), "abc123", start)
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.Code != "access_denied" {
		t.Fatalf("denial error = %v", err)
	}

	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	expiredOAuth, _ := testOAuth(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "d", "user_code": "U", "verification_uri": "https://www.twitch.tv/activate",
				"expires_in": 2, "interval": 1,
			})
		case "/token":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "authorization_pending"})
		default:
			http.NotFound(w, r)
		}
	}, func() time.Time { return now })
	expiredOAuth.Sleep = func(_ context.Context, d time.Duration) error {
		now = now.Add(d)
		return nil
	}
	start, err = expiredOAuth.StartDevice(context.Background(), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	_, err = expiredOAuth.PollDevice(context.Background(), "abc123", start)
	if !errors.As(err, &providerErr) || providerErr.Code != "expired_token" {
		t.Fatalf("expiry error = %v", err)
	}
}

func TestValidateRefreshSubscribeAndSend(t *testing.T) {
	var mu sync.Mutex
	var subscribeBody map[string]any
	var sentBody map[string]string
	token := "access-one"
	oauth, _ := testOAuth(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/validate":
			if r.Header.Get("Authorization") != "OAuth "+token {
				t.Errorf("validate auth = %q", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"client_id": "abc123", "login": "broadcaster", "user_id": "user-1",
				"scopes": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": 3600,
			})
		case "/token":
			if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "refresh_token" {
				t.Errorf("refresh form = %v", r.Form)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-two", "refresh_token": "refresh-two",
				"expires_in": 3600, "scope": []string{ScopeReadChat, ScopeWriteChat},
			})
		case "/helix/eventsub/subscriptions":
			if r.Header.Get("Authorization") != "Bearer access-one" || r.Header.Get("Client-Id") != "abc123" {
				t.Errorf("subscribe headers = %v", r.Header)
			}
			_ = json.NewDecoder(r.Body).Decode(&subscribeBody)
			w.WriteHeader(http.StatusAccepted)
		case "/helix/chat/messages":
			if r.Header.Get("Authorization") != "Bearer access-one" {
				t.Error("send chat auth missing")
			}
			_ = json.NewDecoder(r.Body).Decode(&sentBody)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"is_sent": true}},
			})
		default:
			http.NotFound(w, r)
		}
	}, func() time.Time { return time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC) })
	ctx := context.Background()
	info, err := oauth.Validate(ctx, token)
	if err != nil || info.UserID != "user-1" || info.ClientID != "abc123" || !hasAllScopes(info.Scopes, []string{ScopeReadChat, ScopeWriteChat}) {
		t.Fatalf("validate = %+v, %v", info, err)
	}
	refreshed, err := oauth.Refresh(ctx, "abc123", "refresh-one")
	if err != nil || refreshed.AccessToken != "access-two" || refreshed.RefreshToken != "refresh-two" {
		t.Fatalf("refresh = %+v, %v", refreshed, err)
	}
	creds := Credentials{AccessToken: "access-one", RefreshToken: "refresh-one", ClientID: "abc123", UserID: "user-1"}
	if err := oauth.SubscribeChat(ctx, creds.AccessToken, creds.ClientID, "session-1", creds.UserID); err != nil {
		t.Fatal(err)
	}
	if err := oauth.SendChat(ctx, creds, "user-1", "fixed reply"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if subscribeBody["type"] != "channel.chat.message" {
		t.Fatalf("subscribe body = %+v", subscribeBody)
	}
	condition := subscribeBody["condition"].(map[string]any)
	if condition["broadcaster_user_id"] != "user-1" || condition["user_id"] != "user-1" {
		t.Fatalf("subscribe condition = %+v", condition)
	}
	if sentBody["broadcaster_id"] != "user-1" || sentBody["sender_id"] != "user-1" || sentBody["message"] != "fixed reply" {
		t.Fatalf("send body = %+v", sentBody)
	}
}

func TestSendChatRejectedAndSafeProviderError(t *testing.T) {
	oauth, _ := testOAuth(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/helix/chat/messages" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"is_sent": false, "drop_reason": map[string]string{"code": "msg_ratelimit"}}},
		})
	}, time.Now)
	creds := Credentials{AccessToken: "secret-access", ClientID: "abc123", UserID: "user-1"}
	err := oauth.SendChat(context.Background(), creds, "user-1", "hello")
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.Code != "msg_ratelimit" {
		t.Fatalf("send error = %v", err)
	}
	if strings.Contains(err.Error(), "secret-access") {
		t.Fatal("provider error leaked the access token")
	}
}

func TestCredentialStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := NewCredentialStore(secrets.NewMemory())
	if _, err := store.Load(ctx); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("empty load = %v", err)
	}
	creds := Credentials{AccessToken: "access", RefreshToken: "refresh", ClientID: "abc123", UserID: "user-1", Login: "broadcaster", Scopes: []string{ScopeReadChat, ScopeWriteChat}}
	if err := store.Save(ctx, creds); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(ctx)
	if err != nil || loaded.AccessToken != "access" || loaded.RefreshToken != "refresh" {
		t.Fatalf("load = %+v, %v", loaded, err)
	}
	if err := store.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(ctx); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("after delete = %v", err)
	}
}

func TestValidClientID(t *testing.T) {
	if !ValidClientID("abc123XYZ") {
		t.Fatal("valid client id rejected")
	}
	for _, bad := range []string{"", "has space", "has-dash", strings.Repeat("a", 65)} {
		if ValidClientID(bad) {
			t.Errorf("invalid client id accepted: %q", bad)
		}
	}
}

func TestProviderErrorSafe(t *testing.T) {
	err := &ProviderError{Code: "network", Status: 502}
	encoded, _ := json.Marshal(err)
	text := fmt.Sprintf("%v %+v %#v %s %q %s", err, err, err, err, err, string(encoded))
	if strings.Contains(text, "secret") {
		t.Fatal("provider error unexpectedly contains a secret")
	}
	u, _ := url.Parse("https://www.twitch.tv/activate")
	if u.Scheme != "https" {
		t.Fatal("unreachable")
	}
}
