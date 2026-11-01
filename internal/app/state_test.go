package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"livecheck/internal/domain"
	"livecheck/internal/store"
)

func newStateTestApp(t *testing.T) *App {
	t.Helper()
	a, err := New(context.Background(), Options{
		DataDir:            t.TempDir(),
		ListenAddress:      fmt.Sprintf("127.0.0.1:%d", freePort(t)),
		NoBrowser:          true,
		SessionOnlySecrets: true,
		Stdout:             io.Discard,
		Stderr:             io.Discard,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(a.Close)
	return a
}

func testExpectations(mic string, track int) domain.Expectations {
	return domain.Expectations{
		Microphone:  &domain.InputRef{Name: mic},
		StreamTrack: &track,
	}
}

func nextPrivate(t *testing.T, ch <-chan []byte) domain.PrivateState {
	t.Helper()
	select {
	case data := <-ch:
		var state domain.PrivateState
		if err := json.Unmarshal(data, &state); err != nil {
			t.Fatalf("decode private event: %v", err)
		}
		return state
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for private event")
		return domain.PrivateState{}
	}
}

func nextPublic(t *testing.T, ch <-chan []byte) domain.PublicState {
	t.Helper()
	select {
	case data := <-ch:
		var state domain.PublicState
		if err := json.Unmarshal(data, &state); err != nil {
			t.Fatalf("decode public event: %v", err)
		}
		return state
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for public event")
		return domain.PublicState{}
	}
}

func assertNoEvent(t *testing.T, ch <-chan []byte, label string) {
	t.Helper()
	select {
	case data := <-ch:
		t.Fatalf("%s unexpectedly published: %s", label, data)
	default:
	}
}

func TestConcurrentConflictingSavesYieldOneProductionWinner(t *testing.T) {
	a := newStateTestApp(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := a.SaveSettings(context.Background(), 0, testExpectations(fmt.Sprintf("Mic-%d", i), 1))
			results <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)

	successes, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, domain.ErrRevisionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected save error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want one each", successes, conflicts)
	}
	private, err := a.PrivateSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	public, err := a.PublicSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if private.Settings.Revision != 1 || private.StateRevision != 2 {
		t.Fatalf("private state revision=%d settings=%d, want state=2 settings=1", private.StateRevision, private.Settings.Revision)
	}
	if public.StateRevision != private.StateRevision {
		t.Fatalf("public state revision=%d, private=%d", public.StateRevision, private.StateRevision)
	}
}

func TestProductionSubscriptionsAndPublicationsStayOrdered(t *testing.T) {
	ctx := context.Background()
	a := newStateTestApp(t)

	sub1 := a.SubscribePrivate()
	initial := nextPrivate(t, sub1.C)
	if initial.StateRevision != 1 || initial.Settings.Revision != 0 {
		t.Fatalf("initial private state revision=%d settings=%d", initial.StateRevision, initial.Settings.Revision)
	}
	pub1 := a.SubscribePublic()
	initialPublic := nextPublic(t, pub1.C)
	if initialPublic.StateRevision != 1 {
		t.Fatalf("initial public revision=%d, want 1", initialPublic.StateRevision)
	}

	settings1, err := a.SaveSettings(ctx, 0, testExpectations("Mic-one", 1))
	if err != nil {
		t.Fatal(err)
	}
	private1 := nextPrivate(t, sub1.C)
	public1 := nextPublic(t, pub1.C)
	if private1.StateRevision != 2 || private1.Settings.Revision != 1 || settings1.Revision != 1 {
		t.Fatalf("first publication private=%+v settings=%+v", private1, settings1)
	}
	if public1.StateRevision != 2 {
		t.Fatalf("first public revision=%d, want 2", public1.StateRevision)
	}

	sub2 := a.SubscribePrivate()
	initial2 := nextPrivate(t, sub2.C)
	if initial2.StateRevision != 2 || initial2.Settings.Revision != 1 || initial2.Settings.Microphone == nil || initial2.Settings.Microphone.Name != "Mic-one" {
		t.Fatalf("second subscriber initial state is stale: %+v", initial2)
	}
	pub2 := a.SubscribePublic()
	initialPublic2 := nextPublic(t, pub2.C)
	if initialPublic2.StateRevision != 2 {
		t.Fatalf("second public initial revision=%d, want 2", initialPublic2.StateRevision)
	}

	settings2, err := a.SaveSettings(ctx, 1, testExpectations("Mic-two", 2))
	if err != nil {
		t.Fatal(err)
	}
	update1 := nextPrivate(t, sub1.C)
	update2 := nextPrivate(t, sub2.C)
	publicUpdate1 := nextPublic(t, pub1.C)
	publicUpdate2 := nextPublic(t, pub2.C)
	for label, state := range map[string]domain.PrivateState{"first": update1, "second": update2} {
		if state.StateRevision != 3 || state.Settings.Revision != 2 || state.Settings.Microphone == nil || state.Settings.Microphone.Name != "Mic-two" {
			t.Fatalf("%s subscriber private update regressed: %+v", label, state)
		}
	}
	for label, state := range map[string]domain.PublicState{"first": publicUpdate1, "second": publicUpdate2} {
		if state.StateRevision != 3 {
			t.Fatalf("%s subscriber public update revision=%d, want 3", label, state.StateRevision)
		}
	}
	if settings2.Revision != 2 {
		t.Fatalf("second settings revision=%d, want 2", settings2.Revision)
	}

	// A failed compare-and-update must not publish success or alter cached state.
	if _, err := a.SaveSettings(ctx, 0, testExpectations("Mic-stale", 1)); !errors.Is(err, domain.ErrRevisionConflict) {
		t.Fatalf("stale save error=%v, want revision conflict", err)
	}
	assertNoEvent(t, sub1.C, "first private subscriber after failed save")
	assertNoEvent(t, sub2.C, "second private subscriber after failed save")
	assertNoEvent(t, pub1.C, "first public subscriber after failed save")
	assertNoEvent(t, pub2.C, "second public subscriber after failed save")

	private, err := a.PrivateSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	public, err := a.PublicSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if private.StateRevision != 3 || private.Settings.Revision != 2 || private.Settings.Microphone.Name != "Mic-two" {
		t.Fatalf("cached private state inconsistent: %+v", private)
	}
	if public.StateRevision != 3 {
		t.Fatalf("cached public revision=%d, want 3", public.StateRevision)
	}

	a.Close()
	reopened, err := store.Open(ctx, filepath.Join(a.DataDir(), databaseFileName))
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer reopened.Close()
	persisted, err := reopened.LoadSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Revision != 2 || persisted.Microphone == nil || persisted.Microphone.Name != "Mic-two" {
		t.Fatalf("persisted settings do not match published state: %+v", persisted)
	}
}
