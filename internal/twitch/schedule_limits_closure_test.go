package twitch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/secrets"
)

type closureClock struct {
	mu    sync.Mutex
	value time.Time
}

func (c *closureClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

func (c *closureClock) advance(d time.Duration) {
	c.mu.Lock()
	c.value = c.value.Add(d)
	c.mu.Unlock()
}

func TestScheduledMaintenanceRefreshesBeforeShortTokenExpiry(t *testing.T) {
	for _, lifetime := range []time.Duration{time.Minute, 10 * time.Minute, time.Hour} {
		t.Run(lifetime.String(), func(t *testing.T) {
			clock := &closureClock{value: time.Now()}
			deadline := clock.now().Add(lifetime)
			var refreshes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/validate":
					remaining := int(deadline.Sub(clock.now()).Seconds())
					if r.Header.Get("Authorization") == "OAuth rotated" {
						remaining = 3600
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "abc123", "user_id": "user-1", "scopes": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": remaining})
				case "/token":
					refreshes.Add(1)
					_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "rotated", "refresh_token": "rotated-refresh", "scope": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": 3600})
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			waits, ticks := make(chan time.Duration, 1), make(chan struct{})
			oauth := NewOAuth(server.Client(), testEndpoints(server.URL), clock.now)
			m := NewManager(Options{OAuth: oauth, Store: NewCredentialStore(secrets.NewMemory()), Now: clock.now, MaintenanceWait: func(ctx context.Context, delay time.Duration) bool {
				select {
				case waits <- delay:
				case <-ctx.Done():
					return false
				}
				select {
				case <-ticks:
					return true
				case <-ctx.Done():
					return false
				}
			}})
			defer m.Close()
			seedAuthorization(m)
			m.StartRefreshLoop()
			for refreshes.Load() == 0 {
				var delay time.Duration
				select {
				case delay = <-waits:
				case <-time.After(time.Second):
					t.Fatal("maintenance timer not scheduled")
				}
				// Receiving the next wait also joins the preceding maintenance.
				if refreshes.Load() != 0 {
					break
				}
				if delay > 30*time.Second || delay <= 0 {
					t.Fatalf("unsafe maintenance delay = %v", delay)
				}
				clock.advance(delay)
				if !clock.now().Before(deadline) {
					t.Fatal("scheduled maintenance let token expire")
				}
				ticks <- struct{}{}
			}
			// A refresh count can become visible before validated persistence
			// completes; await the next timer if not already consumed above.
			if got := m.currentCredentials(); got == nil || got.AccessToken != "rotated" {
				select {
				case <-waits:
				case <-time.After(time.Second):
					t.Fatal("scheduled rotation did not complete")
				}
			}
			if got := m.currentCredentials(); got == nil || got.AccessToken != "rotated" || !got.ExpiresAt.After(deadline) {
				t.Fatal("scheduled maintenance did not install the validated rotated pair")
			}
			if refreshes.Load() != 1 {
				t.Fatalf("refreshes=%d, want 1", refreshes.Load())
			}
		})
	}
}

func TestHelixSendHonorsRetryAndExhaustedResetHeaders(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusTooManyRequests} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			clock := &closureClock{value: time.Unix(1800000000, 0)}
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) == 1 {
					w.Header().Set("Retry-After", "2")
					w.Header().Set("Ratelimit-Remaining", "0")
					w.Header().Set("Ratelimit-Reset", fmt.Sprint(clock.now().Add(7*time.Second).Unix()))
					w.WriteHeader(status)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"is_sent": true}}})
			}))
			defer server.Close()
			oauth := NewOAuth(server.Client(), testEndpoints(server.URL), clock.now)
			var waited time.Duration
			oauth.Sleep = func(ctx context.Context, d time.Duration) error { waited += d; clock.advance(d); return ctx.Err() }
			err := oauth.SendChat(context.Background(), reviewCredentials(), "user-1", "first")
			if status == http.StatusOK && err != nil {
				t.Fatal(err)
			}
			if status == http.StatusTooManyRequests {
				provider, ok := err.(*ProviderError)
				if !ok || provider.RetryAfter != 7*time.Second {
					t.Fatalf("rate error=%v", err)
				}
			}
			if delay := oauth.chatDelay(); delay != 7*time.Second {
				t.Fatalf("successful/exhausted bucket delay=%v", delay)
			}
			if err := oauth.SendChat(context.Background(), reviewCredentials(), "user-1", "second"); err != nil {
				t.Fatal(err)
			}
			if waited != 7*time.Second || requests.Load() != 2 {
				t.Fatalf("wait=%v, requests=%d", waited, requests.Load())
			}
		})
	}
}

func TestProviderDelayHeaderVariants(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	for _, tc := range []struct {
		retry, remaining, reset string
		want                    time.Duration
	}{
		{retry: now.Add(5 * time.Second).Format(http.TimeFormat), want: 5 * time.Second},
		{retry: "bad", remaining: "0", reset: "bad"},
		{retry: "-1", remaining: "0", reset: fmt.Sprint(now.Add(-time.Second).Unix())},
		{remaining: "1", reset: fmt.Sprint(now.Add(time.Hour).Unix())},
		{retry: "9", remaining: "0", reset: fmt.Sprint(now.Add(time.Second).Unix()), want: 9 * time.Second},
	} {
		response := &http.Response{Header: make(http.Header)}
		response.Header.Set("Retry-After", tc.retry)
		response.Header.Set("Ratelimit-Remaining", tc.remaining)
		response.Header.Set("Ratelimit-Reset", tc.reset)
		if got := providerDelay(response, now); got != tc.want {
			t.Fatalf("header delay=%v, want %v", got, tc.want)
		}
	}
}

func TestReplyQueueExpiresItemsDuringSuccessfulProviderCooldown(t *testing.T) {
	clock := &closureClock{value: time.Now()}
	first := make(chan struct{})
	var sends atomic.Int32
	queue := NewReplyQueue(context.Background(), func(context.Context, string) error {
		if sends.Add(1) == 1 {
			clock.advance(time.Minute)
			close(first)
		}
		return nil
	}, clock.now, time.Millisecond, func() time.Duration { return 20 * time.Millisecond })
	defer queue.Close()
	// Install both items atomically before waking the worker.
	queue.mu.Lock()
	queue.items = []replyItem{{text: "first", expires: clock.now().Add(30 * time.Second)}, {text: "expired during cooldown", expires: clock.now().Add(30 * time.Second)}}
	queue.mu.Unlock()
	queue.wake <- struct{}{}
	awaitClosed(t, first, "successful first send")
	deadline := time.Now().Add(time.Second)
	for queue.Len() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	queue.Close()
	if sends.Load() != 1 || queue.Len() != 0 {
		t.Fatal("provider cooldown bypassed queue expiry")
	}
}
