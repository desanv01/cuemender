package httpui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"livecheck/internal/domain"
)

const (
	SessionCookieName = "livecheck_session"
	CSRFHeader        = "X-CSRF-Token"
)

// Subscription is one live SSE subscriber. Cancel must be idempotent.
type Subscription struct {
	C      <-chan []byte
	Done   <-chan struct{}
	Cancel func()
}

// Backend is the app-owned state and persistence boundary used by the HTTP
// layer. The public and private methods are deliberately separate so a public
// handler cannot accidentally serialize the private projection.
type Backend interface {
	PrivateSnapshot(context.Context) (domain.PrivateState, error)
	PublicSnapshot(context.Context) (domain.PublicState, error)
	SaveSettings(context.Context, int64, domain.Expectations) (domain.Settings, error)
	SubscribePrivate() Subscription
	SubscribePublic() Subscription
}

type Options struct {
	Backend           Backend
	Host              string
	Port              string
	Mode              string
	Version           string
	Assets            fs.FS
	Now               func() time.Time
	HeartbeatInterval time.Duration
	BootstrapTTL      time.Duration
}

type Server struct {
	backend Backend
	host    string
	port    string
	mode    string
	version string
	now     func() time.Time

	controlFS fs.FS
	overlayFS fs.FS

	bootstrap      *bootstrap
	bootstrapToken string
	sessions       *sessionStore
	limiter        *attemptLimiter
	heartbeat      time.Duration
	handler        http.Handler
	closed         chan struct{}
	closeOnce      sync.Once
	activeStreams  int64
}

func New(opts Options) (*Server, error) {
	if opts.Backend == nil {
		return nil, errors.New("httpui: backend is required")
	}
	if opts.Assets == nil {
		return nil, errors.New("httpui: embedded assets are required")
	}
	ip := net.ParseIP(opts.Host)
	if ip == nil || !ip.IsLoopback() {
		return nil, errors.New("httpui: host must be an explicit loopback IP")
	}
	port, err := strconv.Atoi(opts.Port)
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("httpui: port must be between 1 and 65535")
	}
	controlFS, err := fs.Sub(opts.Assets, "control")
	if err != nil {
		return nil, fmt.Errorf("httpui: control assets: %w", err)
	}
	overlayFS, err := fs.Sub(opts.Assets, "overlay")
	if err != nil {
		return nil, fmt.Errorf("httpui: overlay assets: %w", err)
	}
	if _, err := fs.Stat(controlFS, "index.html"); err != nil {
		return nil, fmt.Errorf("httpui: control/index.html: %w", err)
	}
	if _, err := fs.Stat(overlayFS, "index.html"); err != nil {
		return nil, fmt.Errorf("httpui: overlay/index.html: %w", err)
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}
	mode := opts.Mode
	if mode == "" {
		mode = domain.ModeLive
	}
	heartbeat := opts.HeartbeatInterval
	if heartbeat <= 0 {
		heartbeat = 15 * time.Second
	}
	ttl := opts.BootstrapTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	boot, token, err := newBootstrap(now, ttl)
	if err != nil {
		return nil, fmt.Errorf("httpui: generate bootstrap capability: %w", err)
	}
	s := &Server{
		backend:        opts.Backend,
		host:           opts.Host,
		port:           opts.Port,
		mode:           mode,
		version:        opts.Version,
		now:            now,
		controlFS:      controlFS,
		overlayFS:      overlayFS,
		bootstrap:      boot,
		bootstrapToken: token,
		sessions:       newSessionStore(),
		limiter:        newAttemptLimiter(now()),
		heartbeat:      heartbeat,
		closed:         make(chan struct{}),
	}
	s.handler = s.buildHandler()
	return s, nil
}

func (s *Server) Handler() http.Handler { return s.handler }

// ActiveStreams reports handlers currently serving SSE. It is a diagnostic
// for shutdown tests and logs.
func (s *Server) ActiveStreams() int {
	return int(atomic.LoadInt64(&s.activeStreams))
}

// Close actively ends SSE streams. It is safe to call more than once.
func (s *Server) Close() {
	s.closeOnce.Do(func() { close(s.closed) })
}

// BootstrapToken returns the one-time capability for the launch URL. It must
// never be written to a log or report.
func (s *Server) BootstrapToken() string { return s.bootstrapToken }

func (s *Server) buildHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", onlyMethod(http.MethodGet, http.HandlerFunc(s.handleHealth)).ServeHTTP)
	mux.HandleFunc("/control", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/control/", http.StatusMovedPermanently)
	})
	mux.Handle("/control/", onlyMethod(http.MethodGet, http.StripPrefix("/control/", http.FileServer(http.FS(s.controlFS)))))
	mux.HandleFunc("/overlay", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/overlay/", http.StatusMovedPermanently)
	})
	mux.Handle("/overlay/", onlyMethod(http.MethodGet, http.StripPrefix("/overlay/", http.FileServer(http.FS(s.overlayFS)))))

	mux.HandleFunc("/api/v1/session/bootstrap", onlyMethod(http.MethodPost, http.HandlerFunc(s.handleBootstrap)).ServeHTTP)
	mux.HandleFunc("/api/v1/state", onlyMethod(http.MethodGet, s.requireSession(http.HandlerFunc(s.handleState))).ServeHTTP)
	mux.HandleFunc("/api/v1/settings", onlyMethod(http.MethodPost, s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleSettings)))).ServeHTTP)
	mux.HandleFunc("/api/v1/events", onlyMethod(http.MethodGet, s.requireSession(http.HandlerFunc(s.handlePrivateEvents))).ServeHTTP)

	mux.HandleFunc("/public/v1/state", onlyMethod(http.MethodGet, http.HandlerFunc(s.handlePublicState)).ServeHTTP)
	mux.HandleFunc("/public/v1/events", onlyMethod(http.MethodGet, http.HandlerFunc(s.handlePublicEvents)).ServeHTTP)
	mux.HandleFunc("/", s.handleNotFound)

	return withRequestID(s.withSecurityHeaders(s.withHost(mux)))
}

func (s *Server) withHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowedHost(r.Host) {
			writeError(w, r, http.StatusForbidden, "host_denied", "Request host is not allowed.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		if isPrivatePath(r.URL.Path) {
			h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
			h.Set("X-Frame-Options", "DENY")
		} else if isPublicPath(r.URL.Path) {
			h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors *")
		}
		next.ServeHTTP(w, r)
	})
}

func isPrivatePath(path string) bool {
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/control")
}

func isPublicPath(path string) bool {
	return strings.HasPrefix(path, "/public/") || strings.HasPrefix(path, "/overlay")
}

func (s *Server) allowedHost(host string) bool {
	host = strings.ToLower(host)
	for _, allowed := range s.allowedHosts() {
		if host == allowed {
			return true
		}
	}
	return false
}

func (s *Server) allowedHosts() []string {
	exact := strings.ToLower(net.JoinHostPort(s.host, s.port))
	hosts := []string{exact}
	if ip := net.ParseIP(s.host); ip != nil && ip.IsLoopback() {
		hosts = append(hosts,
			"localhost:"+s.port,
			"127.0.0.1:"+s.port,
			"[::1]:"+s.port,
		)
	}
	return hosts
}

func (s *Server) validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if !strings.EqualFold(u.Host, r.Host) {
		return false
	}
	return s.allowedHost(u.Host)
}

type sessionContextKey struct{}

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "no_session", "A private control session is required.")
			return
		}
		sess, ok := s.sessions.lookup(cookie.Value)
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "no_session", "A private control session is required.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, sess)))
	})
}

func (s *Server) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.validOrigin(r) {
			writeError(w, r, http.StatusForbidden, "origin_denied", "Request origin is not allowed.")
			return
		}
		sess, _ := r.Context().Value(sessionContextKey{}).(session)
		if !constantTimeEqual(r.Header.Get(CSRFHeader), sess.CSRF) {
			writeError(w, r, http.StatusForbidden, "csrf_denied", "Request is missing a valid CSRF token.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	if !s.validOrigin(r) {
		writeError(w, r, http.StatusForbidden, "origin_denied", "Request origin is not allowed.")
		return
	}
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if body.Token == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_bootstrap", "Bootstrap token is required.")
		return
	}
	if err := s.bootstrap.consume(body.Token); err != nil {
		if !s.limiter.allow(s.now()) {
			writeError(w, r, http.StatusTooManyRequests, "rate_limited", "Too many invalid bootstrap attempts. Restart the application for a new link.")
			return
		}
		writeError(w, r, http.StatusUnauthorized, "invalid_bootstrap", "The launch link is invalid, expired or already used.")
		return
	}
	s.limiter.reset(s.now())
	id, csrf, err := s.sessions.create()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "The private session could not be created.")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	writeData(w, r, struct {
		CSRFToken string `json:"csrf_token"`
	}{CSRFToken: csrf})
}

type privateStateResponse struct {
	domain.PrivateState
	CSRFToken string `json:"csrf_token"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	state, err := s.backend.PrivateSnapshot(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "Private state is unavailable.")
		return
	}
	sess, _ := r.Context().Value(sessionContextKey{}).(session)
	writeData(w, r, privateStateResponse{PrivateState: state, CSRFToken: sess.CSRF})
}

type settingsRequest struct {
	ExpectedRevision int64                     `json:"expected_revision"`
	Microphone       *domain.InputRef          `json:"microphone"`
	StreamTrack      *int                      `json:"stream_track"`
	Visual           *domain.VisualExpectation `json:"visual"`
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body settingsRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if body.ExpectedRevision < 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_settings", "expected_revision must be non-negative.")
		return
	}
	exp := domain.Expectations{
		Microphone:  body.Microphone,
		StreamTrack: body.StreamTrack,
		Visual:      body.Visual,
	}
	if err := exp.Validate(); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_settings", "Settings are invalid: "+err.Error())
		return
	}
	settings, err := s.backend.SaveSettings(r.Context(), body.ExpectedRevision, exp)
	if err != nil {
		if errors.Is(err, domain.ErrRevisionConflict) {
			writeError(w, r, http.StatusConflict, "revision_conflict", "Settings changed since they were loaded. Reload and try again.")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal", "Settings could not be saved.")
		return
	}
	writeData(w, r, settings)
}

func (s *Server) handlePublicState(w http.ResponseWriter, r *http.Request) {
	state, err := s.backend.PublicSnapshot(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "Public state is unavailable.")
		return
	}
	writeData(w, r, state)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeData(w, r, struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		Mode    string `json:"mode"`
	}{Status: "ok", Version: s.version, Mode: s.mode})
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusNotFound, "not_found", "This route is not implemented.")
}

func (s *Server) handlePrivateEvents(w http.ResponseWriter, r *http.Request) {
	sub := s.backend.SubscribePrivate()
	defer sub.Cancel()
	s.stream(w, r, sub)
}

func (s *Server) handlePublicEvents(w http.ResponseWriter, r *http.Request) {
	sub := s.backend.SubscribePublic()
	defer sub.Cancel()
	s.stream(w, r, sub)
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request, sub Subscription) {
	atomic.AddInt64(&s.activeStreams, 1)
	defer atomic.AddInt64(&s.activeStreams, -1)
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "streaming_unavailable", "Streaming is not available.")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ticker := time.NewTicker(s.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-s.closed:
			return
		case <-r.Context().Done():
			return
		case <-sub.Done:
			return
		case data, ok := <-sub.C:
			if !ok {
				return
			}
			if err := writeSSE(w, flusher, "state", data); err != nil {
				return
			}
		case <-ticker.C:
			if err := writeHeartbeat(w, flusher); err != nil {
				return
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, data []byte) error {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func writeHeartbeat(w http.ResponseWriter, flusher http.Flusher) error {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
