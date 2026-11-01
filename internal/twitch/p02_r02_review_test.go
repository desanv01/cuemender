// Promoted from Codex's P02-R03 review set; assertions preserved.
package twitch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

type reviewR03HandoverDialer struct {
	socket  Socket
	entered chan struct{}
}

func (d *reviewR03HandoverDialer) Dial(context.Context, string, http.Header) (Socket, error) {
	close(d.entered)
	return d.socket, nil
}
func TestReviewR03HandoverProcessesOldNotifications(t *testing.T) {
	first := newFakeSocket(welcomeFrame("old"), []byte(`{"metadata":{"message_id":"handover","message_type":"session_reconnect"},"payload":{"session":{"reconnect_url":"wss://eventsub.wss.twitch.tv/ws?test=1"}}}`))
	second := newFakeSocket() // Replacement welcome is deliberately delayed.
	dialer := &reviewR03HandoverDialer{socket: second, entered: make(chan struct{})}
	commands := make(chan CommandEvent, 2)
	m := NewManager(Options{Dialer: dialer, OnCommand: func(event CommandEvent) { commands <- event }})
	defer m.Close()
	creds := reviewCredentials()
	m.mu.Lock()
	m.creds = &creds
	m.gen = 1
	m.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- m.runSocket(ctx, 1, first, creds, false) }()
	defer func() { cancel(); <-done; _ = first.Close(); _ = second.Close() }()
	select {
	case <-dialer.entered:
	case <-time.After(time.Second):
		t.Fatal("replacement dial did not start")
	}
	first.ch <- chatFrame("old-delivery", "old-message", "!issueaudio", "user-1", "viewer-2")
	select {
	case <-commands:
	case <-time.After(300 * time.Millisecond):
		t.Error("old-socket notification not processed while replacement welcome is pending")
	}
	second.ch <- welcomeFrame("replacement")
}

func TestReviewR03DisconnectCancelsMaintenance(t *testing.T) {
	entered := make(chan struct{})
	requestDone := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-r.Context().Done():
		case <-release:
		}
		close(requestDone)
	}))
	defer server.Close()
	endpoints := Endpoints{Device: server.URL, Token: server.URL, Validate: server.URL, Helix: server.URL, EventSub: server.URL}
	oauth := NewOAuth(server.Client(), endpoints, time.Now)
	m := NewManager(Options{OAuth: oauth})
	defer m.Close()
	creds := reviewCredentials()
	m.mu.Lock()
	m.creds = &creds
	m.gen = 1
	m.conn.Status = domain.ConnectionConnected
	m.mu.Unlock()
	done := make(chan struct{})
	go func() { m.maintenance(); close(done) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("maintenance did not reach local provider")
	}
	if err := m.Disconnect(context.Background()); err != nil {
		t.Error(err)
	}
	select {
	case <-requestDone:
	case <-time.After(300 * time.Millisecond):
		t.Error("Disconnect returned while its maintenance request remained active")
	}
	close(release)
	<-done
}
