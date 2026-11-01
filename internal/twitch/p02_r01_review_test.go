// Promoted from Codex's P02-R02 review set; assertions preserved in the
// ordinary suite.
package twitch

import (
	"context"
	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/secrets"
	"testing"
	"time"
)

func TestReviewR02MaintenanceExpiryUnits(t *testing.T) {
	oauth, _ := testTwitchOAuthServer(t, "success") // validate returns expires_in=3600 seconds
	now := time.Now()
	m := NewManager(Options{OAuth: oauth, Store: NewCredentialStore(secrets.NewMemory()), Now: func() time.Time { return now }})
	defer m.Close()
	creds := reviewCredentials()
	m.mu.Lock()
	m.creds = &creds
	m.conn.Status = domain.ConnectionConnected
	m.mu.Unlock()
	m.maintenance()
	got := m.currentCredentials().ExpiresAt.Sub(now)
	if got != time.Hour {
		t.Fatalf("3600-second validation produced remaining lifetime %s, want 1h", got)
	}
}

func TestReviewR02RotationSaveFailureUsesRotatedPair(t *testing.T) {
	oauth, _ := testTwitchOAuthServer(t, "success")
	m := NewManager(Options{OAuth: oauth, Store: NewCredentialStore(failingSecrets{})})
	defer m.Close()
	creds := reviewCredentials()
	m.mu.Lock()
	m.creds = &creds
	m.gen = 1
	m.conn.Status = domain.ConnectionConnected
	m.mu.Unlock()
	if err := m.RefreshNow(context.Background()); err == nil {
		t.Fatal("fixture did not fail persistence")
	}
	if !m.Snapshot().CredentialPersistenceDegraded {
		t.Fatal("missing degraded state")
	}
	got := m.currentCredentials()
	if got == nil || got.AccessToken != "refreshed-access" || got.RefreshToken != "rotated-refresh" {
		t.Fatal("validated rotated pair not retained for the established session after persistence failure")
	}
}

func TestReviewR02ReplyQueueRejectsAfterClose(t *testing.T) {
	q := NewReplyQueue(context.Background(), func(context.Context, string) error { return nil }, time.Now, time.Millisecond)
	q.Close()
	if q.Enqueue("fixed copy") {
		t.Fatal("closed reply queue still accepts work")
	}
}
