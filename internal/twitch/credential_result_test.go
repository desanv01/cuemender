package twitch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/secrets"
)

// These barriers separate authorization generation from rotation of credentials
// inside that generation. An old provider response cannot revoke the new pair.
func TestLateCredentialRejectionsDoNotRevokeRotatedPair(t *testing.T) {
	for _, operation := range []string{"maintenance401", "maintenance403", "maintenance_subject", "maintenance_scopes", "maintenance_expiry", "reply", "subscription"} {
		t.Run(operation, func(t *testing.T) {
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			newSubscription := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "rotated", "refresh_token": "rotated-refresh", "expires_in": 3600})
					return
				}
				old := r.Header.Get("Authorization") == "OAuth synthetic-access" || r.Header.Get("Authorization") == "Bearer synthetic-access"
				if old {
					close(entered)
					<-release
					if operation == "maintenance_subject" || operation == "maintenance_scopes" || operation == "maintenance_expiry" {
						user, scopes, expiry := "user-1", []string{ScopeReadChat, ScopeWriteChat}, 3600
						if operation == "maintenance_subject" {
							user = "other"
						}
						if operation == "maintenance_scopes" {
							scopes = nil
						}
						if operation == "maintenance_expiry" {
							expiry = 0
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "abc123", "user_id": user, "scopes": scopes, "expires_in": expiry})
					} else if operation == "maintenance403" {
						w.WriteHeader(http.StatusForbidden)
					} else {
						w.WriteHeader(http.StatusUnauthorized)
					}
					return
				}
				if r.URL.Path == "/helix/eventsub/subscriptions" {
					w.WriteHeader(http.StatusAccepted)
					newSubscription <- struct{}{}
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "abc123", "user_id": "user-1", "scopes": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": 3600})
			}))
			defer server.Close()
			oauth := NewOAuth(server.Client(), testEndpoints(server.URL), time.Now)
			m := NewManager(Options{OAuth: oauth, Store: NewCredentialStore(secrets.NewMemory()), Endpoints: oauth.Endpoints,
				Dialer:    &fakeDialer{sockets: []*fakeSocket{newFakeSocket(welcomeFrame("old")), newFakeSocket(welcomeFrame("new"))}},
				RetryWait: func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil }})
			defer m.Close()
			seedAuthorization(m)
			if operation == "subscription" {
				m.startEventSub(m.authCtx, 1)
			} else if operation == "reply" {
				m.startReplyQueue(1)
				go func() { defer close(done); _ = m.reply.sender(m.authCtx, replyAck) }()
			} else {
				go func() { defer close(done); m.maintenance() }()
			}
			awaitClosed(t, entered, "old credential provider request")
			if err := m.RefreshNow(context.Background()); err != nil {
				close(release)
				t.Fatal(err)
			}
			close(release)
			if operation == "subscription" {
				select {
				case <-newSubscription:
				case <-time.After(time.Second):
					t.Fatal("stale subscription rejection did not reconnect with the rotated pair")
				}
			} else {
				awaitClosed(t, done, "old credential provider result")
			}
			if creds := m.currentCredentials(); creds == nil || creds.AccessToken != "rotated" || m.authCtx.Err() != nil {
				t.Fatal("old credential rejection revoked the validated rotated authorization")
			}
		})
	}
}

func TestOldRejectionWaitsForInflightRotation(t *testing.T) {
	oldEntered, oldRelease, oldReturned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	rotatedEntered, rotatedRelease := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "rotated", "refresh_token": "rotated-refresh", "expires_in": 3600})
			return
		}
		if r.Header.Get("Authorization") == "OAuth synthetic-access" {
			close(oldEntered)
			<-oldRelease
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		close(rotatedEntered)
		<-rotatedRelease
		_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "abc123", "user_id": "user-1", "scopes": []string{ScopeReadChat, ScopeWriteChat}, "expires_in": 3600})
	}))
	defer server.Close()
	oauth := NewOAuth(server.Client(), testEndpoints(server.URL), time.Now)
	m := NewManager(Options{OAuth: oauth, Store: NewCredentialStore(secrets.NewMemory())})
	defer m.Close()
	seedAuthorization(m)
	go func() { m.maintenance(); close(oldReturned) }()
	awaitClosed(t, oldEntered, "old validation")
	rotated := make(chan error, 1)
	go func() { rotated <- m.RefreshNow(context.Background()) }()
	awaitClosed(t, rotatedEntered, "rotation validation")
	close(oldRelease)
	select {
	case <-oldReturned:
		close(rotatedRelease)
		<-rotated
		t.Fatal("old rejection was applied while rotation still owned the credential transition")
	case <-time.After(30 * time.Millisecond):
	}
	close(rotatedRelease)
	if err := <-rotated; err != nil {
		t.Fatal(err)
	}
	awaitClosed(t, oldReturned, "superseded validation")
	if creds := m.currentCredentials(); creds == nil || creds.AccessToken != "rotated" || m.Snapshot().Status != domain.ConnectionConnected {
		t.Fatal("in-flight validated rotation was discarded after an old rejection")
	}
}

func TestDedupCapacityPreservesRetainedDuplicates(t *testing.T) {
	d := NewDedup()
	d.max = 3
	now := time.Now()
	for i := range 3 {
		d.Seen(fmt.Sprint(i), "", now)
	}
	if !d.Seen("0", "", now) {
		t.Fatal("capacity eviction admitted a retained duplicate")
	}
	if d.Seen("new-delivery", "new-message", now) {
		t.Fatal("new pair was a duplicate")
	}
	if len(d.entries) > d.max || !d.Seen("new-delivery", "", now) || !d.Seen("", "new-message", now) {
		t.Fatal("two-ID insertion violated the hard bound or lost the newest pair")
	}
	d = NewDedup()
	d.Seen("same", "same", now)
	if len(d.entries) != 1 || len(d.order) != 1 {
		t.Fatal("equal delivery/message IDs created duplicate order entries")
	}
}
