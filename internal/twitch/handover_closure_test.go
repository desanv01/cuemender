package twitch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type observedSocket struct {
	Socket
	reads  atomic.Int32
	closed atomic.Bool
}

func (s *observedSocket) Read(ctx context.Context) ([]byte, error) {
	s.reads.Add(1)
	defer s.reads.Add(-1)
	return s.Socket.Read(ctx)
}

func (s *observedSocket) Close() error {
	s.closed.Store(true)
	return s.Socket.Close()
}

type localHandoverDialer struct {
	endpoint string
	client   *http.Client
	result   chan *observedSocket
}

func (d localHandoverDialer) Dial(ctx context.Context, endpoint string, header http.Header) (Socket, error) {
	// The production URL check remains intact. Only test transport routing is
	// replaced, while both sockets use the actual coder/websocket adapter.
	if err := ValidateReconnectURL(endpoint); err != nil {
		return nil, err
	}
	sock, err := (CoderDialer{HTTP: d.client}).Dial(ctx, d.endpoint, header)
	if err != nil {
		return nil, err
	}
	observed := &observedSocket{Socket: sock}
	d.result <- observed
	return observed, nil
}

func TestRealWebSocketDelayedHandoverOwnership(t *testing.T) {
	for _, outcome := range []string{"success", "bad_welcome", "cancel", "old_failure"} {
		t.Run(outcome, func(t *testing.T) {
			oldAccepted, newAccepted := make(chan *websocket.Conn, 1), make(chan *websocket.Conn, 1)
			oldClosed, newClosed := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer conn.CloseNow()
				if r.URL.Path == "/old" {
					oldAccepted <- conn
					defer close(oldClosed)
				} else {
					newAccepted <- conn
					defer close(newClosed)
				}
				// Reading detects actual peer transport closure, independent of
				// the client's Close bookkeeping. Server writes happen in test.
				_, _, _ = conn.Read(context.Background())
			}))
			defer server.Close()
			wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
			base, err := (CoderDialer{HTTP: server.Client()}).Dial(context.Background(), wsURL+"/old", nil)
			if err != nil {
				t.Fatal(err)
			}
			old := &observedSocket{Socket: base}
			defer old.Close()
			oldPeer := <-oldAccepted
			dialed := make(chan *observedSocket, 1)
			commands := make(chan CommandEvent, 4)
			oauth, subscriptions := testTwitchOAuthServer(t, "success")
			m := NewManager(Options{
				OAuth: oauth, Dialer: localHandoverDialer{endpoint: wsURL + "/new", client: server.Client(), result: dialed},
				OnCommand: func(event CommandEvent) { commands <- event },
			})
			defer m.Close()
			creds := seedAuthorization(m)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- m.runSocket(ctx, 1, old, creds, true) }()
			defer cancel()
			write := func(conn *websocket.Conn, data []byte) {
				t.Helper()
				writeCtx, stop := context.WithTimeout(context.Background(), time.Second)
				defer stop()
				if err := conn.Write(writeCtx, websocket.MessageText, data); err != nil {
					t.Fatal(err)
				}
			}
			write(oldPeer, welcomeFrame("old"))
			write(oldPeer, []byte(`{"metadata":{"message_id":"handover","message_type":"session_reconnect"},"payload":{"session":{"reconnect_url":"wss://eventsub.wss.twitch.tv/ws?local=1"}}}`))
			var newPeer *websocket.Conn
			select {
			case newPeer = <-newAccepted:
			case <-time.After(2 * time.Second):
				t.Fatal("replacement WebSocket did not connect")
			}
			replacement := <-dialed
			defer replacement.Close()
			// No replacement welcome yet: the old channel remains usable.
			write(oldPeer, chatFrame("old-delivery", "old-message", "!issueaudio", "user-1", "viewer-2"))
			var first CommandEvent
			select {
			case first = <-commands:
			case <-time.After(time.Second):
				t.Fatal("old notification was lost during delayed real-socket welcome")
			}
			switch outcome {
			case "success":
				write(newPeer, welcomeFrame("replacement"))
				awaitClosed(t, oldClosed, "old peer after successful handover")
				write(newPeer, chatFrame("old-delivery", "old-message", "!issueaudio", "user-1", "viewer-2"))
				write(newPeer, chatFrame("new-delivery", "new-message", "!streamhelp", "user-1", "viewer-3"))
				select {
				case event := <-commands:
					if event.MessageID != "new-message" || event.ConnectionGeneration <= first.ConnectionGeneration {
						t.Fatalf("handover duplicate/generation = %+v", event)
					}
				case <-time.After(time.Second):
					t.Fatal("replacement notification missing")
				}
				cancel()
			case "bad_welcome":
				write(newPeer, []byte(`{"metadata":{"message_id":"bad","message_type":"session_keepalive"}}`))
			case "cancel":
				cancel()
			case "old_failure":
				_ = oldPeer.CloseNow()
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("closed transport returned success")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("handover did not join its owned socket work")
			}
			awaitClosed(t, oldClosed, "old peer closure")
			awaitClosed(t, newClosed, "replacement peer closure")
			if !old.closed.Load() || !replacement.closed.Load() || old.reads.Load() != 0 || replacement.reads.Load() != 0 {
				t.Fatal("handover returned with an unclosed socket or active adapter read")
			}
			if subscriptions := atomic.LoadInt32(subscriptions); subscriptions != 1 {
				t.Fatalf("requested handover subscribed %d times", subscriptions)
			}
			want := 1
			if outcome == "success" {
				want = 2
			}
			if got := m.Snapshot().Receipt.Accepted; got != want {
				t.Fatalf("accepted receipts = %d, want %d", got, want)
			}
		})
	}
}

type lateHandoverDialer struct {
	entered   chan struct{}
	cancelled chan struct{}
	release   chan struct{}
	socket    *fakeSocket
}

func (d lateHandoverDialer) Dial(ctx context.Context, _ string, _ http.Header) (Socket, error) {
	close(d.entered)
	<-ctx.Done()
	close(d.cancelled)
	<-d.release // Model a dial completing just as its cancellation arrives.
	return d.socket, nil
}

func TestHandoverOwnsLateDialAfterOldFailure(t *testing.T) {
	old := newFakeSocket()
	next := newFakeSocket(welcomeFrame("late"))
	dialer := lateHandoverDialer{entered: make(chan struct{}), cancelled: make(chan struct{}), release: make(chan struct{}), socket: next}
	m := NewManager(Options{Dialer: dialer})
	defer m.Close()
	seedAuthorization(m)
	done := make(chan error, 1)
	go func() {
		_, _, _, err := m.handover(context.Background(), 1, old, time.Second, "wss://eventsub.wss.twitch.tv/ws")
		done <- err
	}()
	awaitClosed(t, dialer.entered, "delayed dial")
	_ = old.Close()
	awaitClosed(t, dialer.cancelled, "replacement dial cancellation after old failure")
	close(dialer.release)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("failed old socket silently transferred a late replacement")
		}
	case <-time.After(time.Second):
		t.Fatal("late dial not joined")
	}
	select {
	case <-next.closed:
	default:
		t.Fatal("late replacement was leaked")
	}
}

type stableRecoveryDialer struct {
	clock   *closureClock
	calls   int
	stable  Socket
	blocked Socket
}

func (d *stableRecoveryDialer) Dial(context.Context, string, http.Header) (Socket, error) {
	d.calls++
	if d.calls <= 2 {
		return nil, errors.New("temporary dial failure")
	}
	if d.calls == 3 {
		return d.stable, nil
	}
	return d.blocked, nil
}

type stableThenLostSocket struct {
	clock *closureClock
	reads int
}

func (s *stableThenLostSocket) Read(context.Context) ([]byte, error) {
	s.reads++
	if s.reads == 1 {
		return welcomeFrame("stable"), nil
	}
	s.clock.advance(31 * time.Second)
	return nil, errors.New("ordinary loss after stable recovery")
}

func (s *stableThenLostSocket) Close() error { return nil }

func TestReconnectBackoffResetsAfterStableRecovery(t *testing.T) {
	clock := &closureClock{value: time.Now()}
	delays := make(chan time.Duration, 8)
	oauth, _ := testTwitchOAuthServer(t, "success")
	dialer := &stableRecoveryDialer{clock: clock, stable: &stableThenLostSocket{clock: clock}, blocked: newFakeSocket(welcomeFrame("recovered"))}
	m := NewManager(Options{OAuth: oauth, Dialer: dialer, Endpoints: oauth.Endpoints, Now: clock.now, RetryWait: func(ctx context.Context, delay time.Duration) bool { delays <- delay; return ctx.Err() == nil }})
	defer m.Close()
	seedAuthorization(m)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.startEventSub(ctx, 1)
	for _, want := range []time.Duration{time.Second, 2 * time.Second, time.Second} {
		select {
		case got := <-delays:
			if got != want {
				t.Fatalf("reconnect delay = %v, want %v", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("reconnect did not progress")
		}
	}
	cancel()
	m.workerWG.Wait()
}
