package secrets

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestMemoryLifecycleAndIsolation(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	if _, err := m.Get(ctx, "missing"); !IsNotFound(err) {
		t.Fatalf("Get missing error = %v, want not found", err)
	}
	if err := m.Set(ctx, "key", "value"); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get(ctx, "key")
	if err != nil || got != "value" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if err := m.Delete(ctx, "key"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Get(ctx, "key"); !IsNotFound(err) {
		t.Fatalf("after delete error = %v, want not found", err)
	}
	other := NewMemory()
	if _, err := other.Get(ctx, "key"); !IsNotFound(err) {
		t.Fatalf("memory store leaked across instances: %v", err)
	}
}

func TestMemoryHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewMemory().Get(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get canceled error = %v", err)
	}
}

type fakeBackend struct {
	data   map[string]string
	getErr error
	setErr error
	delErr error
}

func (f *fakeBackend) Set(service, user, password string) error {
	if f.setErr != nil {
		return f.setErr
	}
	if f.data == nil {
		f.data = make(map[string]string)
	}
	f.data[service+":"+user] = password
	return nil
}

func (f *fakeBackend) Get(service, user string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	v, ok := f.data[service+":"+user]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return v, nil
}

func (f *fakeBackend) Delete(service, user string) error {
	if f.delErr != nil {
		return f.delErr
	}
	key := service + ":" + user
	if _, ok := f.data[key]; !ok {
		return keyring.ErrNotFound
	}
	delete(f.data, key)
	return nil
}

func TestOSMapsBackendOutcomes(t *testing.T) {
	ctx := context.Background()
	backend := &fakeBackend{}
	store := newOSWithBackend("LiveCheck Test", backend)

	if _, err := store.Get(ctx, "missing"); !IsNotFound(err) {
		t.Fatalf("missing = %v, want not found", err)
	}
	if err := store.Set(ctx, "key", "secret-value"); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "key")
	if err != nil || got != "secret-value" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if !store.Available(ctx) {
		t.Fatal("available should be true when backend returns not found")
	}
	if err := store.Delete(ctx, "key"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "key"); !IsNotFound(err) {
		t.Fatalf("delete missing = %v, want not found", err)
	}

	backend.getErr = errors.New("backend exploded")
	_, err = store.Get(ctx, "key")
	if !IsUnavailable(err) {
		t.Fatalf("get failure = %v, want unavailable", err)
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatal("error leaked a secret value")
	}
	if store.Available(ctx) {
		t.Fatal("available should be false on backend failure")
	}

	backend.getErr = nil
	backend.setErr = errors.New("set exploded")
	err = store.Set(ctx, "key", "another-secret")
	if !IsUnavailable(err) {
		t.Fatalf("set failure = %v, want unavailable", err)
	}
	if strings.Contains(err.Error(), "another-secret") {
		t.Fatal("set error leaked a secret value")
	}
}

func TestCredentialErrorsRedactBackendText(t *testing.T) {
	ctx := context.Background()
	const sentinel = "SENTINEL_BACKEND_CREDENTIAL_4c0ffee"
	backend := &fakeBackend{
		getErr: fmt.Errorf("backend get failed while handling %s", sentinel),
		setErr: fmt.Errorf("backend set failed while handling %s", sentinel),
		delErr: fmt.Errorf("backend delete failed while handling %s", sentinel),
	}
	store := newOSWithBackend("LiveCheck Test", backend)

	assertSafe := func(t *testing.T, err error) {
		t.Helper()
		if !IsUnavailable(err) {
			t.Fatalf("error = %v, want typed unavailable", err)
		}
		formatted := []string{
			err.Error(),
			fmt.Sprintf("%v", err),
			fmt.Sprintf("%+v", err),
			fmt.Sprintf("%#v", err),
			fmt.Sprintf("%s", err),
			fmt.Sprintf("%q", err),
		}
		encoded, jsonErr := json.Marshal(err)
		if jsonErr != nil {
			t.Fatalf("marshal error: %v", jsonErr)
		}
		formatted = append(formatted, string(encoded))
		for _, text := range formatted {
			if strings.Contains(text, sentinel) {
				t.Fatalf("safe formatting leaked the backend sentinel: %q", text)
			}
		}
		var logBuf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuf, nil))
		logger.Error("credential operation failed", "err", err)
		if strings.Contains(logBuf.String(), sentinel) {
			t.Fatalf("structured log leaked the backend sentinel: %q", logBuf.String())
		}
		if errors.Unwrap(err) == nil {
			t.Fatal("expected the backend cause to remain available for programmatic inspection")
		}
	}

	_, err := store.Get(ctx, "token")
	assertSafe(t, err)
	err = store.Set(ctx, "token", "another-secret")
	assertSafe(t, err)
	err = store.Delete(ctx, "token")
	assertSafe(t, err)
	if store.Available(ctx) {
		t.Fatal("available should be false when the backend fails")
	}
}

func TestOSKeyringRoundTrip(t *testing.T) {
	if os.Getenv("LIVECHECK_KEYRING_TEST") != "1" {
		t.Skip("set LIVECHECK_KEYRING_TEST=1 to exercise the real OS keyring")
	}
	ctx := context.Background()
	store := NewOS("LiveCheck P01 Test")
	if !store.Available(ctx) {
		t.Skip("OS keyring is unavailable in this environment")
	}
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	key := "p01-" + hex.EncodeToString(raw)
	value := "disposable-" + hex.EncodeToString(raw)
	t.Cleanup(func() {
		_ = store.Delete(context.Background(), key)
	})
	if err := store.Set(ctx, key, value); err != nil {
		t.Skipf("OS keyring set unavailable: %v", err)
	}
	got, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("real keyring get: %v", err)
	}
	if got != value {
		t.Fatal("real keyring returned the wrong value")
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("real keyring delete: %v", err)
	}
}
