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

	"github.com/desanv01/cuemender/internal/domain"
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
	TwitchConnect(context.Context, string) error
	TwitchDisconnect(context.Context) error
	OBSConnect(context.Context, string, *string) error
	OBSDisconnect(context.Context) error
	OBSDiscovery(context.Context) (domain.OBSDiscovery, error)
	StartMonitoring(context.Context, int64) (domain.MonitoringSessionInfo, error)
	StopMonitoring(context.Context) error
	CheckIncident(context.Context, string, int64, string) (domain.Incident, error)
	DismissIncident(context.Context, string) (domain.Incident, error)
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
	mux.HandleFunc("/api/v1/twitch/connect", onlyMethod(http.MethodPost, s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleTwitchConnect)))).ServeHTTP)
	mux.HandleFunc("/api/v1/twitch/receive-only/connect", onlyMethod(http.MethodPost, s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleReceiveOnlyConnect)))).ServeHTTP)
	mux.HandleFunc("/api/v1/twitch/disconnect", onlyMethod(http.MethodPost, s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleTwitchDisconnect)))).ServeHTTP)
	mux.HandleFunc("/api/v1/obs/connect", onlyMethod(http.MethodPost, s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleOBSConnect)))).ServeHTTP)
	mux.HandleFunc("/api/v1/obs/disconnect", onlyMethod(http.MethodPost, s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleOBSDisconnect)))).ServeHTTP)
	mux.HandleFunc("/api/v1/obs/discovery", onlyMethod(http.MethodGet, s.requireSession(http.HandlerFunc(s.handleOBSDiscovery))).ServeHTTP)
	mux.HandleFunc("/api/v1/monitoring/start", onlyMethod(http.MethodPost, s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleMonitoringStart)))).ServeHTTP)
	mux.HandleFunc("/api/v1/monitoring/stop", onlyMethod(http.MethodPost, s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleMonitoringStop)))).ServeHTTP)
	mux.HandleFunc("/api/v1/incidents/check", onlyMethod(http.MethodPost, s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleIncidentCheck)))).ServeHTTP)
	mux.Handle("POST /api/v1/proposals/{id}/approve", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleApprove))))
	mux.Handle("POST /api/v1/incidents/{id}/dismiss", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleIncidentDismiss))))
	mux.Handle("POST /api/v1/incidents/{id}/close", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleIncidentClose))))
	mux.Handle("POST /api/v1/rehearsal/scenario", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleRehearsalScenario))))
	mux.Handle("POST /api/v1/rehearsal/report", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleRehearsalReport))))
	mux.Handle("POST /api/v1/rehearsal/feedback", s.requireSession(s.requireCSRF(http.HandlerFunc(s.handleRehearsalFeedback))))

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

func (s *Server) handleTwitchConnect(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct {
		ClientID string `json:"client_id"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if strings.TrimSpace(body.ClientID) == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_client_id", "A Twitch Public client ID is required.")
		return
	}
	if err := s.backend.TwitchConnect(r.Context(), body.ClientID); err != nil {
		writeIntegrationError(w, r, err)
		return
	}
	writeDataStatus(w, r, http.StatusAccepted, map[string]string{"status": "authorizing"})
}

func (s *Server) handleTwitchDisconnect(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct{}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if err := s.backend.TwitchDisconnect(r.Context()); err != nil {
		writeIntegrationError(w, r, err)
		return
	}
	writeData(w, r, map[string]string{"status": "disconnected"})
}

func (s *Server) handleOBSConnect(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct {
		Address  string  `json:"address"`
		Password *string `json:"password"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if strings.TrimSpace(body.Address) == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_obs_address", "An OBS WebSocket address is required.")
		return
	}
	if err := s.backend.OBSConnect(r.Context(), body.Address, body.Password); err != nil {
		writeIntegrationError(w, r, err)
		return
	}
	writeDataStatus(w, r, http.StatusAccepted, map[string]string{"status": "connecting"})
}

func (s *Server) handleOBSDisconnect(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct{}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if err := s.backend.OBSDisconnect(r.Context()); err != nil {
		writeIntegrationError(w, r, err)
		return
	}
	writeData(w, r, map[string]string{"status": "disconnected"})
}

func (s *Server) handleOBSDiscovery(w http.ResponseWriter, r *http.Request) {
	discovery, err := s.backend.OBSDiscovery(r.Context())
	if err != nil {
		writeIntegrationError(w, r, err)
		return
	}
	writeData(w, r, discovery)
}

func (s *Server) handleMonitoringStart(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct {
		ExpectedRevision int64 `json:"expected_revision"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if body.ExpectedRevision < 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_revision", "expected_revision must be non-negative.")
		return
	}
	session, err := s.backend.StartMonitoring(r.Context(), body.ExpectedRevision)
	if err != nil {
		writeIncidentError(w, r, err)
		return
	}
	writeData(w, r, map[string]any{"monitoring_active": session.Active, "session_id": session.ID})
}

func (s *Server) handleMonitoringStop(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct{}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if err := s.backend.StopMonitoring(r.Context()); err != nil {
		writeIncidentError(w, r, err)
		return
	}
	writeData(w, r, map[string]any{"monitoring_active": false, "session_id": nil})
}

func (s *Server) handleIncidentCheck(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct {
		Category         string `json:"category"`
		ExpectedRevision int64  `json:"expected_revision"`
		IncidentID       string `json:"incident_id"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if body.Category != "audio" && body.Category != "video" {
		writeError(w, r, http.StatusBadRequest, "invalid_category", "category must be audio or video.")
		return
	}
	if body.ExpectedRevision < 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_revision", "expected_revision must be non-negative.")
		return
	}
	inc, err := s.backend.CheckIncident(r.Context(), body.Category, body.ExpectedRevision, body.IncidentID)
	if err != nil {
		writeIncidentError(w, r, err)
		return
	}
	writeDataStatus(w, r, http.StatusAccepted, map[string]any{
		"incident_id": inc.ID, "public_id": inc.PublicID, "stage": inc.Stage,
	})
}

func (s *Server) handleIncidentDismiss(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct{}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_incident_id", "incident id is required.")
		return
	}
	inc, err := s.backend.DismissIncident(r.Context(), id)
	if err != nil {
		writeIncidentError(w, r, err)
		return
	}
	writeData(w, r, map[string]any{"incident_id": inc.ID, "stage": inc.Stage})
}

func (s *Server) handleIncidentClose(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct {
		Outcome string `json:"outcome"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if body.Outcome != "viewer_confirmed" && body.Outcome != "local_only" && body.Outcome != "unresolved" {
		writeError(w, r, http.StatusBadRequest, "invalid_closure_outcome", "outcome must be viewer_confirmed, local_only or unresolved.")
		return
	}
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_incident_id", "incident id is required.")
		return
	}
	backend, ok := s.backend.(interface {
		CloseIncident(context.Context, string, string) (domain.Incident, error)
	})
	if !ok {
		writeError(w, r, http.StatusServiceUnavailable, "unavailable", "Closure is unavailable.")
		return
	}
	inc, err := backend.CloseIncident(r.Context(), id, body.Outcome)
	if err != nil {
		writeIncidentError(w, r, err)
		return
	}
	writeData(w, r, inc)
}

func (s *Server) handleRehearsalScenario(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct {
		Scenario string `json:"scenario"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	switch body.Scenario {
	case "track_missing", "mic_muted", "wrong_scene", "unknown_write":
	default:
		writeError(w, r, http.StatusBadRequest, "invalid_scenario", "Unknown rehearsal scenario.")
		return
	}
	backend, ok := s.backend.(interface {
		RehearsalScenario(context.Context, string) (domain.Settings, error)
	})
	if !ok {
		writeError(w, r, http.StatusConflict, "mode_conflict", "Rehearsal is unavailable in live mode.")
		return
	}
	settings, err := backend.RehearsalScenario(r.Context(), body.Scenario)
	if err != nil {
		writeIncidentError(w, r, err)
		return
	}
	writeData(w, r, settings)
}

func (s *Server) handleRehearsalReport(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct {
		Category string `json:"category"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if body.Category != "audio" && body.Category != "video" {
		writeError(w, r, http.StatusBadRequest, "invalid_category", "category must be audio or video.")
		return
	}
	backend, ok := s.backend.(interface {
		RehearsalReport(context.Context, string) (domain.Incident, error)
	})
	if !ok {
		writeError(w, r, http.StatusConflict, "mode_conflict", "Rehearsal is unavailable in live mode.")
		return
	}
	inc, err := backend.RehearsalReport(r.Context(), body.Category)
	if err != nil {
		writeIncidentError(w, r, err)
		return
	}
	writeDataStatus(w, r, http.StatusAccepted, inc)
}

func (s *Server) handleRehearsalFeedback(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct {
		PublicID string `json:"public_id"`
		Vote     string `json:"vote"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if body.PublicID == "" || (body.Vote != "fixed" && body.Vote != "stillbroken") {
		writeError(w, r, http.StatusBadRequest, "invalid_feedback", "public_id and a valid vote are required.")
		return
	}
	backend, ok := s.backend.(interface {
		RehearsalFeedback(context.Context, string, string) (domain.Incident, error)
	})
	if !ok {
		writeError(w, r, http.StatusConflict, "mode_conflict", "Rehearsal is unavailable in live mode.")
		return
	}
	inc, err := backend.RehearsalFeedback(r.Context(), body.PublicID, body.Vote)
	if err != nil {
		writeIncidentError(w, r, err)
		return
	}
	writeDataStatus(w, r, http.StatusAccepted, inc)
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(w, r); err != nil {
		writeClientError(w, r, err)
		return
	}
	var body struct {
		ExpectedRevision *int64 `json:"expected_revision"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeClientError(w, r, err)
		return
	}
	if body.ExpectedRevision == nil || *body.ExpectedRevision < 0 {
		writeError(w, r, 400, "invalid_revision", "expected_revision is required and must be non-negative.")
		return
	}
	backend, ok := s.backend.(interface {
		ApproveProposal(context.Context, string, int64) (domain.Action, error)
	})
	if !ok {
		writeError(w, r, 503, "unavailable", "Approved repair is unavailable.")
		return
	}
	action, err := backend.ApproveProposal(r.Context(), r.PathValue("id"), *body.ExpectedRevision)
	if err != nil {
		writeIncidentError(w, r, err)
		return
	}
	writeDataStatus(w, r, http.StatusAccepted, action)
}

func writeIncidentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrProposalNotFound):
		writeError(w, r, 404, "proposal_not_found", "Proposal was not found.")
	case errors.Is(err, domain.ErrProposalExpired):
		writeError(w, r, 410, "proposal_expired", "The proposal expired. Run a fresh check.")
	case errors.Is(err, domain.ErrActionConflict):
		writeError(w, r, 409, "action_conflict", "The proposal or action is no longer eligible. Run a fresh check when the pending action settles.")
	case errors.Is(err, domain.ErrRevisionConflict):
		writeError(w, r, http.StatusConflict, "revision_conflict", "Settings changed since they were loaded.")
	case errors.Is(err, domain.ErrNotReady):
		writeError(w, r, http.StatusConflict, "monitoring_not_ready", "Monitoring prerequisites are not ready.")
	case errors.Is(err, domain.ErrJobLimit), errors.Is(err, domain.ErrBusy):
		writeError(w, r, http.StatusTooManyRequests, "diagnostic_busy", "A diagnostic job is already running for that category.")
	case errors.Is(err, domain.ErrIncidentNotFound):
		writeError(w, r, http.StatusNotFound, "incident_not_found", "Incident was not found.")
	case errors.Is(err, domain.ErrTerminalIncident):
		writeError(w, r, http.StatusConflict, "incident_terminal", "That incident is terminal or belongs to another session.")
	case errors.Is(err, domain.ErrClosureIneligible):
		writeError(w, r, http.StatusConflict, "closure_ineligible", "That incident is not eligible for closure while work is pending.")
	case errors.Is(err, domain.ErrClosureConflict):
		writeError(w, r, http.StatusConflict, "closure_conflict", "That incident was already closed with a different outcome.")
	case errors.Is(err, domain.ErrClosureFeedback):
		writeError(w, r, http.StatusConflict, "closure_feedback", "Viewer confirmation or local verification is not eligible for this outcome.")
	case errors.Is(err, domain.ErrModeConflict):
		writeError(w, r, http.StatusConflict, "mode_conflict", "This operation is unavailable in the current runtime mode.")
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_request", "The request is invalid.")
	case errors.Is(err, domain.ErrUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "unavailable", "The requested service is unavailable.")
	default:
		writeError(w, r, http.StatusInternalServerError, "internal", "The operation failed.")
	}
}

func writeIntegrationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrModeConflict):
		writeError(w, r, http.StatusConflict, "mode_conflict", "This operation is unavailable for the selected connection mode.")
	case errors.Is(err, domain.ErrBusy):
		writeError(w, r, http.StatusConflict, "integration_busy", "That integration is already busy.")
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_integration_input", "The integration settings are invalid.")
	case errors.Is(err, domain.ErrCredentialStorage):
		writeError(w, r, http.StatusServiceUnavailable, "credential_storage_unavailable", "Credential storage is unavailable. Restart with --session-only-secrets if appropriate.")
	case errors.Is(err, domain.ErrUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "integration_unavailable", "That integration is unavailable.")
	default:
		writeError(w, r, http.StatusInternalServerError, "internal", "The integration operation failed.")
	}
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
