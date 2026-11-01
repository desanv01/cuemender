package obs

import (
	"context"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/secrets"
)

// Corrected R02 coverage: an established transport loss invalidates data,
// advances the generation, reconnects and installs a fresh observation.
func TestOBSLossReconnectsWithNewGeneration(t *testing.T) {
	backend := &reviewOBSBackend{}
	manager := NewManager(Options{
		Dialer:       reviewOBSDialer{backend},
		Secrets:      secrets.NewMemory(),
		PollInterval: 10 * time.Millisecond,
		OnUpdate:     func() {},
	})
	t.Cleanup(manager.Close)
	manager.SetExpectations(testSettings())
	password := ""
	if err := manager.Connect("ws://127.0.0.1:4455", &password); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, manager)
	before := manager.Snapshot()
	backend.broken.Store(true)
	deadline := time.Now().Add(2 * time.Second)
	for manager.Snapshot().Status == domain.ConnectionConnected && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if manager.Snapshot().Status == domain.ConnectionConnected {
		t.Fatal("connection loss stayed connected")
	}
	backend.broken.Store(false)
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := manager.Snapshot()
		if snapshot.Status == domain.ConnectionConnected && manager.Observation() != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	after := manager.Snapshot()
	if after.Status != domain.ConnectionConnected || manager.Observation() == nil {
		t.Fatalf("reconnect did not restore a fresh observation: %+v", after)
	}
	if after.Generation <= before.Generation {
		t.Fatalf("generation did not advance on loss: before=%d after=%d", before.Generation, after.Generation)
	}
}

func TestOBSManualDisconnectStopsReconnect(t *testing.T) {
	backend := &reviewOBSBackend{}
	manager := NewManager(Options{
		Dialer:       reviewOBSDialer{backend},
		Secrets:      secrets.NewMemory(),
		PollInterval: 10 * time.Millisecond,
		OnUpdate:     func() {},
	})
	t.Cleanup(manager.Close)
	manager.SetExpectations(testSettings())
	password := ""
	if err := manager.Connect("ws://127.0.0.1:4455", &password); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, manager)
	backend.broken.Store(true)
	deadline := time.Now().Add(2 * time.Second)
	for manager.Snapshot().Status == domain.ConnectionConnected && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if err := manager.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if manager.Snapshot().Status != domain.ConnectionNotConnected {
		t.Fatalf("manual disconnect did not stop retry: %s", manager.Snapshot().Status)
	}
}

func TestOBSSnapshotBundleIsRevisionCoherent(t *testing.T) {
	manager, _, _, _ := newFakeManager(t)
	manager.pollInterval = 10 * time.Millisecond
	if err := manager.Connect("ws://127.0.0.1:4455", nil); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, manager)
	manager.SetSettings(1, testSettings())
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, _, observation, ready := manager.SnapshotBundle(domain.Settings{Revision: 1, Expectations: testSettings()})
		if ready && observation != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	_, _, observation, ready := manager.SnapshotBundle(domain.Settings{Revision: 1, Expectations: testSettings()})
	if !ready || observation == nil || observation.ConfigRevision != 1 {
		t.Fatalf("revision 1 bundle not coherent: ready=%v obs=%+v", ready, observation)
	}
	manager.SetSettings(2, testSettings())
	if _, _, _, ready := manager.SnapshotBundle(domain.Settings{Revision: 1, Expectations: testSettings()}); ready {
		t.Fatal("old settings revision remained ready after save")
	}
	if _, _, observation, _ := manager.SnapshotBundle(domain.Settings{Revision: 2, Expectations: testSettings()}); observation != nil {
		t.Fatal("old observation survived settings revision change")
	}
}

func TestOBSCollectionChangeRequiresReselection(t *testing.T) {
	manager, backend, _, _ := newFakeManager(t)
	backend.mu.Lock()
	backend.collection = "Collection A"
	backend.mu.Unlock()
	settings := testSettings()
	settings.Microphone.Context = "Collection A"
	settings.Visual.Source.Context = "Collection A"
	manager.SetSettings(1, settings)
	if err := manager.Connect("ws://127.0.0.1:4455", nil); err != nil {
		t.Fatal(err)
	}
	waitConnected(t, manager)
	if _, _, _, ready := manager.SnapshotBundle(domain.Settings{Revision: 1, Expectations: settings}); !ready {
		t.Fatal("collection-bound setup was not ready")
	}
	backend.mu.Lock()
	backend.collection = "Collection B"
	backend.mu.Unlock()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if discovery := manager.Discovery(); discovery != nil && discovery.SceneCollection == "Collection B" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, _, _, ready := manager.SnapshotBundle(domain.Settings{Revision: 1, Expectations: settings}); ready {
		t.Fatal("collection change inherited stale readiness")
	}
}
