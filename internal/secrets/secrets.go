package secrets

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/zalando/go-keyring"
)

// Store is the narrow credential boundary. Values must never be included in
// formatted errors or logs.
type Store interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}

type ErrorKind string

const (
	KindNotFound    ErrorKind = "not_found"
	KindUnavailable ErrorKind = "unavailable"
)

// Error is a typed secret-store failure. Op, Key and Kind are safe metadata;
// the backend cause is retained only for programmatic inspection and is never
// included in formatted text.
type Error struct {
	Kind  ErrorKind
	Op    string
	Key   string
	cause error
}

func (e *Error) Error() string {
	if e.Op == "" {
		return fmt.Sprintf("secret %s (%s)", e.Key, e.Kind)
	}
	return fmt.Sprintf("secret %s %s (%s)", e.Op, e.Key, e.Kind)
}

// Format prevents %+v, %#v or other verbs from exposing the retained backend
// cause. fmt uses this method for the error in normal log formatting.
func (e *Error) Format(f fmt.State, verb rune) {
	if verb == 'q' {
		fmt.Fprintf(f, "%q", e.Error())
		return
	}
	fmt.Fprint(f, e.Error())
}

func (e *Error) Unwrap() error { return e.cause }

func IsNotFound(err error) bool {
	var target *Error
	return errors.As(err, &target) && target.Kind == KindNotFound
}

func IsUnavailable(err error) bool {
	var target *Error
	return errors.As(err, &target) && target.Kind == KindUnavailable
}

// Memory is an explicit process-lifetime implementation. It is never used as
// an automatic fallback for a failed OS keyring.
type Memory struct {
	mu   sync.Mutex
	data map[string]string
}

func NewMemory() *Memory {
	return &Memory{data: make(map[string]string)}
}

func (m *Memory) Get(ctx context.Context, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.data[key]
	if !ok {
		return "", &Error{Kind: KindNotFound, Op: "get", Key: key}
	}
	return value, nil
}

func (m *Memory) Set(ctx context.Context, key, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *Memory) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[key]; !ok {
		return &Error{Kind: KindNotFound, Op: "delete", Key: key}
	}
	delete(m.data, key)
	return nil
}

type keyringBackend interface {
	Set(service, user, password string) error
	Get(service, user string) (string, error)
	Delete(service, user string) error
}

type goKeyringBackend struct{}

func (goKeyringBackend) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}
func (goKeyringBackend) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}
func (goKeyringBackend) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

// OS uses the platform keyring through zalando/go-keyring.
type OS struct {
	service string
	backend keyringBackend
}

func NewOS(service string) *OS {
	return &OS{service: service, backend: goKeyringBackend{}}
}

func newOSWithBackend(service string, backend keyringBackend) *OS {
	return &OS{service: service, backend: backend}
}

func (o *OS) Get(ctx context.Context, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	value, err := o.backend.Get(o.service, key)
	if err == nil {
		return value, nil
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return "", &Error{Kind: KindNotFound, Op: "get", Key: key}
	}
	return "", &Error{Kind: KindUnavailable, Op: "get", Key: key, cause: err}
}

func (o *OS) Set(ctx context.Context, key, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := o.backend.Set(o.service, key, value); err != nil {
		return &Error{Kind: KindUnavailable, Op: "set", Key: key, cause: err}
	}
	return nil
}

func (o *OS) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := o.backend.Delete(o.service, key); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return &Error{Kind: KindNotFound, Op: "delete", Key: key}
		}
		return &Error{Kind: KindUnavailable, Op: "delete", Key: key, cause: err}
	}
	return nil
}

// Available probes the OS keyring without storing anything. A not-found
// result proves the backend answered and is usable.
func (o *OS) Available(ctx context.Context) bool {
	_, err := o.Get(ctx, "availability-probe")
	return err == nil || IsNotFound(err)
}
