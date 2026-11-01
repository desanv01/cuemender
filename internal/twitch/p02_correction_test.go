package twitch

import (
	"context"
	"errors"
	"testing"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/secrets"
)

func TestManagerRefusesUnavailableCredentialStorage(t *testing.T) {
	manager := NewManager(Options{
		Context:     context.Background(),
		Store:       NewCredentialStore(secrets.NewMemory()),
		StorageMode: domain.SecretsModeUnavailable,
	})
	t.Cleanup(manager.Close)
	if err := manager.Connect("abc123"); !errors.Is(err, ErrCredentialStorageUnavailable) {
		t.Fatalf("connect error = %v", err)
	}
	if manager.Snapshot().Status != domain.ConnectionNotConnected {
		t.Fatalf("unavailable storage started a connection: %s", manager.Snapshot().Status)
	}
}

func TestManagerEventSubRevocationIsTerminal(t *testing.T) {
	oauth, _ := testTwitchOAuthServer(t, "success")
	socket := newFakeSocket(
		welcomeFrame("revocation-session"),
		[]byte(`{"metadata":{"message_id":"revocation-1","message_type":"revocation"},"payload":{"subscription":{"type":"channel.chat.message","status":"authorization_revoked"}}}`),
	)
	manager := NewManager(Options{
		Context:   context.Background(),
		OAuth:     oauth,
		Store:     NewCredentialStore(secrets.NewMemory()),
		Endpoints: oauth.Endpoints,
	})
	t.Cleanup(manager.Close)
	creds := reviewCredentials()
	manager.mu.Lock()
	manager.creds = &creds
	manager.gen = 1
	manager.mu.Unlock()
	err := manager.runSocket(context.Background(), 1, socket, creds, false)
	if !errors.Is(err, ErrAuthorizationRevoked) {
		t.Fatalf("revocation error = %v", err)
	}
}
