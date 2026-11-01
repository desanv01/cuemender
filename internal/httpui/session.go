package httpui

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var errInvalidBootstrap = errors.New("invalid or expired bootstrap capability")

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type bootstrap struct {
	mu      sync.Mutex
	hash    [32]byte
	expires time.Time
	used    bool
	now     func() time.Time
}

func newBootstrap(now func() time.Time, ttl time.Duration) (*bootstrap, string, error) {
	token, err := randomHex(32)
	if err != nil {
		return nil, "", err
	}
	return &bootstrap{
		hash:    sha256.Sum256([]byte(token)),
		expires: now().Add(ttl),
		now:     now,
	}, token, nil
}

func (b *bootstrap) consume(token string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.used || !b.now().Before(b.expires) {
		return errInvalidBootstrap
	}
	sum := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(sum[:], b.hash[:]) != 1 {
		return errInvalidBootstrap
	}
	b.used = true
	return nil
}

type attemptLimiter struct {
	mu     sync.Mutex
	window time.Time
	count  int
	limit  int
	period time.Duration
}

func newAttemptLimiter(now time.Time) *attemptLimiter {
	return &attemptLimiter{window: now, limit: 10, period: time.Minute}
}

func (l *attemptLimiter) allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.window) >= l.period {
		l.window = now
		l.count = 0
	}
	l.count++
	return l.count <= l.limit
}

func (l *attemptLimiter) reset(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.count = 0
	l.window = now
}

type session struct {
	CSRF string
}

type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]session
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string]session)}
}

func (s *sessionStore) create() (id, csrf string, err error) {
	id, err = randomHex(32)
	if err != nil {
		return "", "", err
	}
	csrf, err = randomHex(32)
	if err != nil {
		return "", "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = session{CSRF: csrf}
	return id, csrf, nil
}

func (s *sessionStore) lookup(id string) (session, bool) {
	if id == "" {
		return session{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.sessions[id]
	return v, ok
}

// constantTimeEqual compares two fixed-length strings without early exit.
func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
