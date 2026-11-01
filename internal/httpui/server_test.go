package httpui_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/httpui"
	"github.com/desanv01/cuemender/internal/store"
)

type testBackend struct {
	store *store.Store
	now   func() time.Time

	mu            sync.Mutex
	settings      domain.Settings
	stateRevision uint64
	privateHub    *testHub
	publicHub     *testHub
	discovery     *domain.OBSDiscovery
	twitchClient  string
	obsAddress    string
	obsPassword   *string
	routeErr      error
	monitoring    bool
	incidents     []domain.Incident
}

func newTestBackend(t *testing.T) *testBackend {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	settings, err := st.LoadSettings(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	return &testBackend{
		store:         st,
		now:           time.Now,
		settings:      settings,
		stateRevision: 1,
		privateHub:    newTestHub(),
		publicHub:     newTestHub(),
	}
}

func (b *testBackend) PrivateSnapshot(context.Context) (domain.PrivateState, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.privateLocked(), nil
}

func (b *testBackend) PublicSnapshot(context.Context) (domain.PublicState, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.publicLocked(), nil
}

func (b *testBackend) SaveSettings(ctx context.Context, expected int64, exp domain.Expectations) (domain.Settings, error) {
	if exp.StreamTrack != nil {
		confirmed := b.now().UTC()
		exp.TrackConfirmedAt = &confirmed
	} else {
		exp.TrackConfirmedAt = nil
	}
	settings, err := b.store.SaveSettings(ctx, expected, exp)
	if err != nil {
		return domain.Settings{}, err
	}
	b.mu.Lock()
	b.settings = settings
	b.stateRevision++
	privateJSON, _ := json.Marshal(b.privateLocked())
	publicJSON, _ := json.Marshal(b.publicLocked())
	b.mu.Unlock()
	b.privateHub.publish(privateJSON)
	b.publicHub.publish(publicJSON)
	return settings, nil
}

func (b *testBackend) SubscribePrivate() httpui.Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, _ := json.Marshal(b.privateLocked())
	return b.privateHub.subscribe(data)
}

func (b *testBackend) SubscribePublic() httpui.Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, _ := json.Marshal(b.publicLocked())
	return b.publicHub.subscribe(data)
}

func (b *testBackend) privateLocked() domain.PrivateState {
	return domain.PrivateState{
		Mode:             domain.ModeLive,
		StateRevision:    b.stateRevision,
		MonitoringActive: b.monitoring,
		Incidents:        b.incidents,
		Settings:         b.settings,
		Setup: domain.SetupReadiness{
			MicrophoneConfigured:  b.settings.Microphone != nil,
			StreamTrackConfigured: b.settings.StreamTrack != nil,
			VisualConfigured:      b.settings.Visual != nil,
		},
		Secrets:      domain.SecretsStatus{Mode: domain.SecretsModeOS, Available: true},
		Twitch:       domain.TwitchConnection{Status: domain.ConnectionNotConnected},
		OBS:          domain.OBSConnection{Status: domain.ConnectionNotConnected},
		OBSDiscovery: b.discovery,
		Startup:      domain.StartupInfo{ListenAddress: "127.0.0.1:8787", OBSAddress: "ws://127.0.0.1:4455"},
	}
}

func (b *testBackend) TwitchConnect(_ context.Context, clientID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.twitchClient = clientID
	return b.routeErr
}
func (b *testBackend) TwitchDisconnect(context.Context) error { return b.routeErr }
func (b *testBackend) OBSConnect(_ context.Context, address string, password *string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.obsAddress = address
	b.obsPassword = password
	return b.routeErr
}
func (b *testBackend) OBSDisconnect(context.Context) error { return b.routeErr }
func (b *testBackend) OBSDiscovery(context.Context) (domain.OBSDiscovery, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.discovery == nil {
		return domain.OBSDiscovery{}, domain.ErrUnavailable
	}
	return *b.discovery, nil
}

func (b *testBackend) StartMonitoring(_ context.Context, revision int64) (domain.MonitoringSessionInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.routeErr != nil {
		return domain.MonitoringSessionInfo{}, b.routeErr
	}
	if revision != b.settings.Revision {
		return domain.MonitoringSessionInfo{}, domain.ErrRevisionConflict
	}
	b.monitoring = true
	b.stateRevision++
	return domain.MonitoringSessionInfo{ID: "sess-test", Active: true, StartedAt: b.now().UTC()}, nil
}

func (b *testBackend) StopMonitoring(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.monitoring = false
	return b.routeErr
}

func (b *testBackend) CheckIncident(_ context.Context, category string, _ int64, incidentID string) (domain.Incident, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.routeErr != nil {
		return domain.Incident{}, b.routeErr
	}
	if category != "audio" && category != "video" {
		return domain.Incident{}, domain.ErrInvalidInput
	}
	if incidentID != "" {
		for _, inc := range b.incidents {
			if inc.ID == incidentID {
				return inc, nil
			}
		}
		return domain.Incident{}, domain.ErrIncidentNotFound
	}
	return domain.Incident{ID: "inc-test", PublicID: "CM-TEST", Category: category, Stage: domain.StageInvestigating}, nil
}

func (b *testBackend) DismissIncident(_ context.Context, incidentID string) (domain.Incident, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.routeErr != nil {
		return domain.Incident{}, b.routeErr
	}
	for i := range b.incidents {
		if b.incidents[i].ID == incidentID {
			b.incidents[i].Stage = domain.StageDismissed
			return b.incidents[i], nil
		}
	}
	if incidentID == "inc-test" {
		return domain.Incident{ID: "inc-test", Stage: domain.StageDismissed}, nil
	}
	return domain.Incident{}, domain.ErrIncidentNotFound
}

func (b *testBackend) publicLocked() domain.PublicState {
	public := make([]domain.PublicIncident, 0, len(b.incidents))
	for _, inc := range b.incidents {
		if inc.PublicID != "" {
			public = append(public, domain.PublicIncident{PublicID: inc.PublicID, Category: inc.Category, Stage: inc.Stage, MessageCode: "finding_available"})
		}
	}
	return domain.PublicState{
		Mode:          domain.ModeLive,
		StateRevision: b.stateRevision,
		ServiceStatus: domain.ServiceDisconnected,
		Incidents:     public,
	}
}

type testHub struct {
	mu   sync.Mutex
	next uint64
	subs map[uint64]chan []byte
	done map[uint64]chan struct{}
}

func newTestHub() *testHub {
	return &testHub{subs: make(map[uint64]chan []byte), done: make(map[uint64]chan struct{})}
}

func (h *testHub) subscribe(initial []byte) httpui.Subscription {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next++
	id := h.next
	ch := make(chan []byte, 1)
	ch <- initial
	done := make(chan struct{})
	h.subs[id] = ch
	h.done[id] = done
	return httpui.Subscription{
		C:    ch,
		Done: done,
		Cancel: func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if _, ok := h.subs[id]; !ok {
				return
			}
			delete(h.subs, id)
			close(h.done[id])
			delete(h.done, id)
		},
	}
}

func (h *testHub) publish(data []byte) {
	h.mu.Lock()
	subs := make([]chan []byte, 0, len(h.subs))
	for _, ch := range h.subs {
		subs = append(subs, ch)
	}
	h.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- data:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- data:
			default:
			}
		}
	}
}

func testAssets() fs.FS {
	return fstest.MapFS{
		"control/index.html": {Data: []byte("<!doctype html><title>control</title><script src=\"app.js\"></script>")},
		"control/app.js":     {Data: []byte("'use strict';")},
		"control/style.css":  {Data: []byte("body{}")},
		"overlay/index.html": {Data: []byte("<!doctype html><title>overlay</title><script src=\"app.js\"></script>")},
		"overlay/app.js":     {Data: []byte("'use strict';")},
		"overlay/style.css":  {Data: []byte("body{}")},
	}
}

func newTestServer(t *testing.T, now func() time.Time) (*httptest.Server, *httpui.Server, *testBackend) {
	t.Helper()
	backend := newTestBackend(t)
	if now != nil {
		backend.now = now
	}
	ts := httptest.NewUnstartedServer(nil)
	host, port, err := net.SplitHostPort(ts.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ui, err := httpui.New(httpui.Options{
		Backend:           backend,
		Host:              host,
		Port:              port,
		Mode:              domain.ModeLive,
		Version:           "test",
		Assets:            testAssets(),
		Now:               backend.now,
		HeartbeatInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("httpui.New: %v", err)
	}
	ts.Config.Handler = ui.Handler()
	ts.Start()
	t.Cleanup(ts.Close)
	return ts, ui, backend
}

func newClient(t *testing.T, ts *httptest.Server) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := ts.Client()
	client.Jar = jar
	return client
}

func doJSON(t *testing.T, client *http.Client, method, url string, body string, headers map[string]string) (*http.Response, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	var parsed map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	return resp, parsed
}

func bootstrap(t *testing.T, ts *httptest.Server, client *http.Client, token string) string {
	t.Helper()
	resp, body := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/session/bootstrap",
		`{"token":"`+token+`"}`, map[string]string{"Origin": ts.URL})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap status = %d, body = %v", resp.StatusCode, body)
	}
	data, _ := body["data"].(map[string]any)
	csrf, _ := data["csrf_token"].(string)
	if csrf == "" {
		t.Fatal("bootstrap did not return csrf_token")
	}
	return csrf
}

func settingsJSON(revision int64, mic string, track int, scene string) string {
	return fmt.Sprintf(`{"expected_revision":%d,"microphone":{"name":%q,"uuid":"uuid-1"},"stream_track":%d,"visual":{"scene":%q,"scene_item_id":"42","source":{"name":"Visual Source"}}}`,
		revision, mic, track, scene)
}

func TestBootstrapSessionAndHostBoundary(t *testing.T) {
	ts, ui, _ := newTestServer(t, nil)
	client := newClient(t, ts)

	resp, _ := doJSON(t, client, http.MethodGet, ts.URL+"/api/v1/state", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated state = %d, want 401", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodGet, ts.URL+"/api/v1/events", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated private SSE = %d, want 401", resp.StatusCode)
	}

	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/session/bootstrap",
		`{"token":"wrong"}`, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("bootstrap without Origin = %d, want 403", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/session/bootstrap",
		`{"token":"wrong"}`, map[string]string{"Origin": "http://evil.example"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("bootstrap with foreign Origin = %d, want 403", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/session/bootstrap",
		`{"token":"wrong"}`, map[string]string{"Origin": ts.URL})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid bootstrap token = %d, want 401", resp.StatusCode)
	}

	// A valid bootstrap returns a session cookie and CSRF token.
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/session/bootstrap", strings.NewReader(`{"token":"`+ui.BootstrapToken()+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", ts.URL)
	valid, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer valid.Body.Close()
	if valid.StatusCode != http.StatusOK {
		t.Fatalf("valid bootstrap = %d", valid.StatusCode)
	}
	cookies := valid.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("bootstrap set %d cookies, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != httpui.SessionCookieName || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Fatalf("unexpected session cookie: %+v", cookie)
	}
	var body map[string]any
	_ = json.NewDecoder(valid.Body).Decode(&body)
	csrf := body["data"].(map[string]any)["csrf_token"].(string)
	if csrf == "" {
		t.Fatal("missing csrf token")
	}

	resp, stateBody := doJSON(t, client, http.MethodGet, ts.URL+"/api/v1/state", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authenticated state = %d", resp.StatusCode)
	}
	stateData := stateBody["data"].(map[string]any)
	if stateData["csrf_token"] != csrf {
		t.Fatal("state csrf token does not match bootstrap token")
	}
	if stateData["settings"].(map[string]any)["revision"].(float64) != 0 {
		t.Fatal("fresh settings revision is not 0")
	}

	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/session/bootstrap",
		`{"token":"`+ui.BootstrapToken()+`"}`, map[string]string{"Origin": ts.URL})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("replayed bootstrap = %d, want 401", resp.StatusCode)
	}

	// Foreign Host is rejected even when the socket is loopback.
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/healthz", nil)
	req.Host = "evil.example"
	hostResp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	hostResp.Body.Close()
	if hostResp.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign Host = %d, want 403", hostResp.StatusCode)
	}
}

func TestSettingsRevisionValidationAndPublicAllowlist(t *testing.T) {
	ts, ui, _ := newTestServer(t, nil)
	client := newClient(t, ts)
	csrf := bootstrap(t, ts, client, ui.BootstrapToken())
	sentinel := "SENTINEL_PRIVATE_MIC_9f3c"

	resp, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/settings",
		settingsJSON(0, sentinel, 1, "Private Scene"), map[string]string{"Origin": ts.URL, "X-CSRF-Token": csrf, "Content-Type": "application/json"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("valid settings save = %d", resp.StatusCode)
	}

	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/settings",
		settingsJSON(0, sentinel, 1, "Private Scene"), map[string]string{"Origin": ts.URL, "X-CSRF-Token": csrf, "Content-Type": "application/json"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("stale settings save = %d, want 409", resp.StatusCode)
	}

	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/settings",
		settingsJSON(1, sentinel, 7, "Private Scene"), map[string]string{"Origin": ts.URL, "X-CSRF-Token": csrf, "Content-Type": "application/json"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid track = %d, want 400", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/settings",
		`{"expected_revision":1,"unknown_field":true}`, map[string]string{"Origin": ts.URL, "X-CSRF-Token": csrf, "Content-Type": "application/json"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown field = %d, want 400", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/settings",
		settingsJSON(1, sentinel, 1, "Private Scene"), map[string]string{"Origin": ts.URL, "X-CSRF-Token": csrf, "Content-Type": "text/plain"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-json content type = %d, want 400", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/settings",
		settingsJSON(1, sentinel, 1, "Private Scene"), map[string]string{"Origin": ts.URL, "X-CSRF-Token": "wrong"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("bad CSRF = %d, want 403", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/settings",
		settingsJSON(1, sentinel, 1, "Private Scene"), map[string]string{"Origin": "http://evil.example", "X-CSRF-Token": csrf})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("bad Origin = %d, want 403", resp.StatusCode)
	}
	oversized := `{"expected_revision":1,"padding":"` + strings.Repeat("x", 70<<10) + `"}`
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/settings",
		oversized, map[string]string{"Origin": ts.URL, "X-CSRF-Token": csrf})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("oversized body = %d, want 400", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodGet, ts.URL+"/api/v1/settings", "", nil)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET mutation route = %d, want 405", resp.StatusCode)
	}

	resp, _ = doJSON(t, client, http.MethodGet, ts.URL+"/api/v1/state", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("private state = %d", resp.StatusCode)
	}
	resp, publicBody := doJSON(t, client, http.MethodGet, ts.URL+"/public/v1/state", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("public state = %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS header unexpectedly present")
	}
	publicJSON, _ := json.Marshal(publicBody)
	if strings.Contains(string(publicJSON), sentinel) {
		t.Fatal("public state leaked the private source name")
	}
	if strings.Contains(string(publicJSON), "csrf_token") {
		t.Fatal("public state leaked csrf_token")
	}
	sseCtx, sseCancel := context.WithCancel(context.Background())
	defer sseCancel()
	sseResp := openSSE(t, client, ts.URL+"/public/v1/events", sseCtx)
	defer sseResp.Body.Close()
	sseEvent := newSSEReader(sseResp.Body).next(t)
	if strings.Contains(string(sseEvent.Data), sentinel) || strings.Contains(string(sseEvent.Data), "csrf_token") {
		t.Fatal("public SSE leaked private data")
	}
}

func TestPrivateStateContainsSavedExpectations(t *testing.T) {
	ts, ui, _ := newTestServer(t, nil)
	client := newClient(t, ts)
	csrf := bootstrap(t, ts, client, ui.BootstrapToken())
	resp, body := doJSON(t, client, http.MethodGet, ts.URL+"/api/v1/state", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("state = %d", resp.StatusCode)
	}
	initialRevision := body["data"].(map[string]any)["settings"].(map[string]any)["revision"].(float64)
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/settings",
		settingsJSON(int64(initialRevision), "Mic/Aux", 2, "Main"), map[string]string{"Origin": ts.URL, "X-CSRF-Token": csrf})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save = %d", resp.StatusCode)
	}
	resp, body = doJSON(t, client, http.MethodGet, ts.URL+"/api/v1/state", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("state after save = %d", resp.StatusCode)
	}
	data := body["data"].(map[string]any)
	settings := data["settings"].(map[string]any)
	if settings["revision"].(float64) != 1 {
		t.Fatalf("settings revision = %v, want 1", settings["revision"])
	}
	if settings["microphone"].(map[string]any)["name"] != "Mic/Aux" {
		t.Fatal("microphone missing from private state")
	}
	if settings["track_confirmed_at"] == nil {
		t.Fatal("track_confirmed_at was not recorded")
	}
	if data["setup"].(map[string]any)["microphone_configured"] != true {
		t.Fatal("setup readiness did not update")
	}
}

func TestSSEInitialUpdateCancellationAndSlowReader(t *testing.T) {
	ts, ui, _ := newTestServer(t, nil)
	client := newClient(t, ts)
	csrf := bootstrap(t, ts, client, ui.BootstrapToken())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	publicResp := openSSE(t, client, ts.URL+"/public/v1/events", ctx)
	defer publicResp.Body.Close()
	publicReader := newSSEReader(publicResp.Body)
	first := publicReader.next(t)
	var publicState domain.PublicState
	if err := json.Unmarshal(first.Data, &publicState); err != nil {
		t.Fatalf("decode public SSE: %v", err)
	}
	if publicState.StateRevision != 1 {
		t.Fatalf("initial public state revision = %d, want 1", publicState.StateRevision)
	}

	privateCtx, privateCancel := context.WithCancel(context.Background())
	privateResp := openSSE(t, client, ts.URL+"/api/v1/events", privateCtx)
	defer privateResp.Body.Close()
	privateReader := newSSEReader(privateResp.Body)
	firstPrivate := privateReader.next(t)
	if firstPrivate.Name != "state" || !strings.Contains(string(firstPrivate.Data), `"settings"`) {
		t.Fatal("private SSE initial snapshot did not contain settings")
	}
	privateCancel()

	// Keep one public subscriber that never reads. It must not stall the app.
	slowCtx, slowCancel := context.WithCancel(context.Background())
	slowResp := openSSE(t, client, ts.URL+"/public/v1/events", slowCtx)
	defer slowResp.Body.Close()
	_ = slowResp

	resp, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/settings",
		settingsJSON(0, "Mic/Aux", 2, "Main"), map[string]string{"Origin": ts.URL, "X-CSRF-Token": csrf})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save = %d", resp.StatusCode)
	}
	update := publicReader.next(t)
	var updated domain.PublicState
	if err := json.Unmarshal(update.Data, &updated); err != nil {
		t.Fatalf("decode update: %v", err)
	}
	if updated.StateRevision != 2 {
		t.Fatalf("public update revision = %d, want 2", updated.StateRevision)
	}

	// A second subscriber still receives current state while the slow one is stuck.
	secondCtx, secondCancel := context.WithCancel(context.Background())
	defer secondCancel()
	secondResp := openSSE(t, client, ts.URL+"/public/v1/events", secondCtx)
	defer secondResp.Body.Close()
	secondEvent := newSSEReader(secondResp.Body).next(t)
	var secondState domain.PublicState
	if err := json.Unmarshal(secondEvent.Data, &secondState); err != nil {
		t.Fatal(err)
	}
	if secondState.StateRevision != 2 {
		t.Fatalf("second subscriber revision = %d, want 2", secondState.StateRevision)
	}

	health, _ := doJSON(t, client, http.MethodGet, ts.URL+"/healthz", "", nil)
	if health.StatusCode != http.StatusOK {
		t.Fatalf("healthz with slow SSE = %d", health.StatusCode)
	}
	slowCancel()
	cancel()
}

func TestBootstrapExpiryAndRateLimit(t *testing.T) {
	now := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	ts, ui, _ := newTestServer(t, func() time.Time { return now })
	client := newClient(t, ts)
	now = now.Add(6 * time.Minute)
	resp, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/session/bootstrap",
		`{"token":"`+ui.BootstrapToken()+`"}`, map[string]string{"Origin": ts.URL})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired bootstrap = %d, want 401", resp.StatusCode)
	}

	ts2, _, _ := newTestServer(t, nil)
	client2 := newClient(t, ts2)
	for i := 0; i < 10; i++ {
		resp, _ := doJSON(t, client2, http.MethodPost, ts2.URL+"/api/v1/session/bootstrap",
			`{"token":"wrong"}`, map[string]string{"Origin": ts2.URL})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401", i, resp.StatusCode)
		}
	}
	resp, _ = doJSON(t, client2, http.MethodPost, ts2.URL+"/api/v1/session/bootstrap",
		`{"token":"wrong"}`, map[string]string{"Origin": ts2.URL})
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("rate limit = %d, want 429", resp.StatusCode)
	}
}

func TestStaticAssetsAndSecurityHeaders(t *testing.T) {
	ts, _, _ := newTestServer(t, nil)
	client := newClient(t, ts)
	resp, _ := doJSON(t, client, http.MethodGet, ts.URL+"/control/", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("control = %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatal("private CSP missing frame-ancestors none")
	}
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatal("private control missing X-Frame-Options")
	}
	resp, _ = doJSON(t, client, http.MethodGet, ts.URL+"/overlay/", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("overlay = %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Frame-Options") != "" {
		t.Fatal("public overlay must remain embeddable")
	}
	resp, body := doJSON(t, client, http.MethodGet, ts.URL+"/healthz", "", nil)
	if resp.StatusCode != http.StatusOK || body["data"].(map[string]any)["status"] != "ok" {
		t.Fatalf("healthz = %d %v", resp.StatusCode, body)
	}
	if resp.Header.Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID header")
	}
	resp, _ = doJSON(t, client, http.MethodGet, ts.URL+"/nope", "", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown route = %d, want 404", resp.StatusCode)
	}
}

func TestP02PrivateRoutesBoundaryAndRedaction(t *testing.T) {
	ts, ui, backend := newTestServer(t, nil)
	client := newClient(t, ts)
	csrf := bootstrap(t, ts, client, ui.BootstrapToken())
	headers := map[string]string{"Origin": ts.URL, "X-CSRF-Token": csrf}

	resp, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/twitch/connect",
		`{"client_id":"publicclient123"}`, headers)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("twitch connect = %d, want 202", resp.StatusCode)
	}
	backend.mu.Lock()
	gotClient := backend.twitchClient
	backend.mu.Unlock()
	if gotClient != "publicclient123" {
		t.Fatalf("twitch client = %q", gotClient)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/twitch/connect",
		`{"client_id":"publicclient123","unknown":true}`, headers)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown field = %d, want 400", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodGet, ts.URL+"/api/v1/twitch/connect", "", headers)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET connect = %d, want 405", resp.StatusCode)
	}
	resp, _ = doJSON(t, &http.Client{}, http.MethodPost, ts.URL+"/api/v1/twitch/connect",
		`{"client_id":"publicclient123"}`, headers)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-session connect = %d, want 401", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/twitch/connect",
		`{"client_id":"publicclient123"}`, map[string]string{"Origin": ts.URL})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("missing CSRF = %d, want 403", resp.StatusCode)
	}

	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/obs/connect",
		`{"address":"ws://127.0.0.1:4455","password":"private-obs-password"}`, headers)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("obs connect = %d, want 202", resp.StatusCode)
	}
	backend.mu.Lock()
	if backend.obsAddress != "ws://127.0.0.1:4455" || backend.obsPassword == nil || *backend.obsPassword != "private-obs-password" {
		t.Fatalf("obs connect body not propagated: %q %v", backend.obsAddress, backend.obsPassword)
	}
	backend.mu.Unlock()
	resp, _ = doJSON(t, client, http.MethodGet, ts.URL+"/api/v1/obs/discovery", "", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unavailable discovery = %d, want 503", resp.StatusCode)
	}
	sentinel := "SENTINEL_OBS_SOURCE_7f21"
	backend.mu.Lock()
	backend.discovery = &domain.OBSDiscovery{
		Generation: 3,
		CapturedAt: time.Now().UTC(),
		Inputs:     []domain.OBSInput{{Name: sentinel, UUID: "uuid-sentinel"}},
		Scenes:     []domain.OBSScene{{Name: "Private Scene", UUID: "scene-sentinel", Items: []domain.OBSSceneItem{{ID: 4, SourceName: sentinel}}}},
	}
	backend.mu.Unlock()
	resp, body := doJSON(t, client, http.MethodGet, ts.URL+"/api/v1/obs/discovery", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("discovery = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(fmt.Sprint(body), sentinel) {
		t.Fatal("private discovery did not contain the source sentinel")
	}
	resp, publicBody := doJSON(t, client, http.MethodGet, ts.URL+"/public/v1/state", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("public state = %d", resp.StatusCode)
	}
	publicJSON, _ := json.Marshal(publicBody)
	if strings.Contains(string(publicJSON), sentinel) {
		t.Fatal("public state leaked an OBS source name")
	}
	sseCtx, sseCancel := context.WithCancel(context.Background())
	defer sseCancel()
	sseResp := openSSE(t, client, ts.URL+"/public/v1/events", sseCtx)
	defer sseResp.Body.Close()
	sseEvent := newSSEReader(sseResp.Body).next(t)
	if strings.Contains(string(sseEvent.Data), sentinel) {
		t.Fatal("public SSE leaked an OBS source name")
	}

	backend.mu.Lock()
	backend.routeErr = domain.ErrBusy
	backend.mu.Unlock()
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/twitch/disconnect", `{}`, headers)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("busy disconnect = %d, want 409", resp.StatusCode)
	}
	backend.mu.Lock()
	backend.routeErr = nil
	backend.mu.Unlock()
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/twitch/disconnect", `{}`, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("twitch disconnect = %d, want 200", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/v1/obs/disconnect", `{}`, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("obs disconnect = %d, want 200", resp.StatusCode)
	}
}

func openSSE(t *testing.T, client *http.Client, url string, ctx context.Context) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("open SSE: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("SSE status = %d, want 200", resp.StatusCode)
	}
	return resp
}

type sseEvent struct {
	Name string
	Data []byte
}

type sseReader struct {
	r *bufio.Reader
}

func newSSEReader(r io.Reader) *sseReader {
	return &sseReader{r: bufio.NewReader(r)}
}

func (s *sseReader) next(t *testing.T) sseEvent {
	t.Helper()
	type result struct {
		event sseEvent
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		var eventName string
		var data strings.Builder
		for {
			line, err := s.r.ReadString('\n')
			if err != nil {
				ch <- result{err: err}
				return
			}
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			if line == "" {
				if eventName != "" || data.Len() > 0 {
					ch <- result{event: sseEvent{Name: eventName, Data: []byte(data.String())}}
					return
				}
				continue
			}
			if strings.HasPrefix(line, ":") {
				continue
			}
			if strings.HasPrefix(line, "event: ") {
				eventName = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				if data.Len() > 0 {
					data.WriteByte('\n')
				}
				data.WriteString(strings.TrimPrefix(line, "data: "))
			}
		}
	}()
	select {
	case got := <-ch:
		if got.err != nil {
			t.Fatalf("read SSE: %v", got.err)
		}
		return got.event
	case <-time.After(3 * time.Second):
		t.Fatal("timed out reading SSE event")
		return sseEvent{}
	}
}
