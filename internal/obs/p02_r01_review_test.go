// Promoted from Codex's P02-R02 review set; assertions preserved in the
// ordinary suite.
package obs

import (
	"context"
	"errors"
	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/secrets"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReviewR02ActualSDKReadCancellation(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server, _ := startOBSProtocolServer(t, func(id, typ string) map[string]any {
		once.Do(func() { close(entered) })
		<-release
		return map[string]any{"op": 7, "d": map[string]any{"requestType": typ, "requestId": id, "requestStatus": map[string]any{"result": true, "code": 100}, "responseData": map[string]any{"inputMuted": true}}}
	})
	backend, err := (GoobsDialer{}).Connect(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http"), "")
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { _, err := backend.InputMute(ctx, domain.InputRef{Name: "Review mic"}); result <- err }()
	<-entered
	cancel()
	select {
	case <-result:
	case <-time.After(200 * time.Millisecond):
		t.Error("cancelled actual SDK request remains blocked until transport is manually closed")
		_ = backend.Close()
		<-result
	}
	close(release)
	_ = backend.Close()
}

type reviewInitialBackend struct {
	fakeBackend
	entered      chan struct{}
	release      chan struct{}
	once         sync.Once
	enteredOnce  sync.Once
	blockVersion bool
}

func (b *reviewInitialBackend) Version(ctx context.Context) (VersionInfo, error) {
	if b.blockVersion {
		b.enteredOnce.Do(func() { close(b.entered) })
		<-b.release
	}
	return b.fakeBackend.Version(ctx)
}
func (b *reviewInitialBackend) InputMute(ctx context.Context, ref domain.InputRef) (*bool, error) {
	if !b.blockVersion {
		b.enteredOnce.Do(func() { close(b.entered) })
		<-b.release
	}
	return b.fakeBackend.InputMute(ctx, ref)
}
func (b *reviewInitialBackend) Close() error {
	b.once.Do(func() { close(b.release) })
	return b.fakeBackend.Close()
}
func TestReviewR02DisconnectClosesInitialReadTransport(t *testing.T) {
	b := &reviewInitialBackend{entered: make(chan struct{}), release: make(chan struct{}), blockVersion: true}
	m := NewManager(Options{Dialer: reviewOBSDialer{b}, Secrets: secrets.NewMemory()})
	defer m.Close()
	if err := m.Connect("ws://127.0.0.1:4455", nil); err != nil {
		t.Fatal(err)
	}
	<-b.entered
	done := make(chan error, 1)
	go func() { done <- m.Disconnect(context.Background()) }()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Error("disconnect cannot close transport while initial Version read is pending")
		_ = b.Close()
		<-done
	}
}
func TestReviewR02InitialReadRejectsOldSettings(t *testing.T) {
	b := &reviewInitialBackend{entered: make(chan struct{}), release: make(chan struct{})}
	m := NewManager(Options{Dialer: reviewOBSDialer{b}, Secrets: secrets.NewMemory(), PollInterval: time.Hour})
	defer m.Close()
	m.SetSettings(1, testSettings())
	if err := m.Connect("ws://127.0.0.1:4455", nil); err != nil {
		t.Fatal(err)
	}
	<-b.entered
	m.SetSettings(2, testSettings())
	_ = b.Close()
	waitConnected(t, m)
	_, _, observation, _ := m.SnapshotBundle(domain.Settings{Revision: 2, Expectations: testSettings()})
	if observation != nil && observation.ConfigRevision != 2 {
		t.Fatalf("old initial-read revision %d exposed under settings revision 2", observation.ConfigRevision)
	}
}

type reviewMissingCollection struct{ fakeBackend }

func (b *reviewMissingCollection) SceneCollection(context.Context) (string, error) {
	return "", errors.New("collection read unavailable")
}
func TestReviewR02UnknownCollectionCannotResolve(t *testing.T) {
	b := &reviewMissingCollection{}
	m := NewManager(Options{Dialer: reviewOBSDialer{b}, Secrets: secrets.NewMemory(), PollInterval: time.Hour})
	defer m.Close()
	settings := testSettings()
	settings.Microphone.Context = "Collection A"
	settings.Visual.Source.Context = "Collection A"
	m.SetSettings(1, settings)
	if err := m.Connect("ws://127.0.0.1:4455", nil); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, m)
	if _, _, _, ready := m.SnapshotBundle(domain.Settings{Revision: 1, Expectations: settings}); ready {
		t.Fatal("failed collection read bypasses the configured context and reports resolution ready")
	}
}
