// Promoted from Codex's P02-R03 review set; assertions preserved.
package obs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	gorilla "github.com/gorilla/websocket"
)

func TestReviewR03OBSHelloCancellation(t *testing.T) {
	accepted := make(chan *gorilla.Conn, 1)
	peerDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&gorilla.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer close(peerDone)
		defer conn.Close()
		accepted <- conn
		// WebSocket upgrade succeeds, but OBS Hello never arrives.
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		b, err := (GoobsDialer{}).Connect(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), "")
		if b != nil {
			_ = b.Close()
		}
		done <- err
	}()
	var peer *gorilla.Conn
	select {
	case peer = <-accepted:
	case <-time.After(time.Second):
		t.Fatal("fixture did not upgrade")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Error("cancelled OBS connect remains blocked waiting for Hello")
		_ = peer.Close()
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Fatal("connect did not finish after fixture cleanup")
		}
	}
	select {
	case <-peerDone:
	case <-time.After(300 * time.Millisecond):
		t.Error("cancelled OBS connect left its socket open")
		_ = peer.Close()
		<-peerDone
	}
}

func TestReviewR03OBSStaleCannotBeReady(t *testing.T) {
	var nanos atomic.Int64
	nanos.Store(time.Now().UnixNano())
	b := &fakeBackend{collection: "Collection A"}
	m := NewManager(Options{Dialer: reviewOBSDialer{b}, PollInterval: time.Hour, StaleAfter: time.Second, Now: func() time.Time { return time.Unix(0, nanos.Load()) }})
	defer m.Close()
	settings := testSettings()
	settings.Microphone.Context = "Collection A"
	settings.Visual.Source.Context = "Collection A"
	m.SetSettings(1, settings)
	if err := m.Connect("ws://127.0.0.1:4455", nil); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, m)
	_, _, _, ready := m.SnapshotBundle(domain.Settings{Revision: 1, Expectations: settings})
	if !ready {
		t.Fatal("fixture did not establish ready state")
	}
	nanos.Add(int64(2 * time.Second))
	conn, _, _, ready := m.SnapshotBundle(domain.Settings{Revision: 1, Expectations: settings})
	if !conn.Stale {
		t.Fatal("fixture did not advance beyond stale limit")
	}
	if ready {
		t.Fatal("stale observation still reports resolution ready")
	}
}

type reviewR03CollectionRecovery struct {
	fakeBackend
	calls int
}

func (b *reviewR03CollectionRecovery) SceneCollection(context.Context) (string, error) {
	b.calls++
	if b.calls == 1 {
		return "", errors.New("first context read failed")
	}
	return "Collection A", nil
}
func TestReviewR03OBSUnknownStartContextStaysUnknown(t *testing.T) {
	m := NewManager(Options{})
	defer m.Close()
	d, err := m.readDiscovery(context.Background(), 0, &reviewR03CollectionRecovery{})
	if err != nil {
		return
	} // Rejecting the whole ambiguous discovery is also valid.
	if d.CollectionKnown {
		t.Fatal("successful final collection read upgrades an unknown starting context to known")
	}
}

type reviewR03SlowInputs struct {
	fakeBackend
	entered, release chan struct{}
}

func (b *reviewR03SlowInputs) Inputs(ctx context.Context) ([]domain.OBSInput, error) {
	close(b.entered)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.release:
	}
	return b.fakeBackend.Inputs(ctx)
}
func TestReviewR03OBSRefreshCannotRelabelRevision(t *testing.T) {
	b := &reviewR03SlowInputs{entered: make(chan struct{}), release: make(chan struct{})}
	m := NewManager(Options{})
	defer m.Close()
	m.SetSettings(1, testSettings())
	m.mu.Lock()
	m.backend = b
	m.gen = 1
	m.conn.Status = domain.ConnectionConnected
	m.mu.Unlock()
	done := make(chan *domain.OBSDiscovery, 1)
	go func() { d, _ := m.RefreshDiscovery(context.Background()); done <- d }()
	<-b.entered
	m.SetSettings(2, testSettings())
	close(b.release)
	d := <-done
	if d != nil {
		t.Errorf("read begun under revision 1 returned discovery labelled revision %d after revision changed", d.ConfigRevision)
	}
	if m.Discovery() != nil {
		t.Error("old in-flight discovery installed after settings change")
	}
}
