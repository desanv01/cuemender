package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/httpui"
	"github.com/desanv01/cuemender/internal/secrets"
	"github.com/desanv01/cuemender/internal/store"
	"github.com/desanv01/cuemender/web"
)

type Options struct {
	DataDir            string
	ListenAddress      string
	NoBrowser          bool
	SessionOnlySecrets bool
	Stdout             io.Writer
	Stderr             io.Writer
	Now                func() time.Time
}

type App struct {
	cfg     Config
	dataDir string
	now     func() time.Time

	listener     net.Listener
	store        *store.Store
	secrets      secrets.Store
	secretsState domain.SecretsStatus

	ui         *httpui.Server
	httpServer *http.Server
	stdout     io.Writer
	stderr     io.Writer
	noBrowser  bool
	logger     *slog.Logger

	stateMu       sync.RWMutex
	saveMu        sync.Mutex
	settings      domain.Settings
	stateRevision uint64
	privateHub    *hub
	publicHub     *hub

	connMu sync.Mutex
	conns  map[net.Conn]http.ConnState

	baseCtx    context.Context
	baseCancel context.CancelFunc
	closeOnce  sync.Once

	baseURL    string
	controlURL string
}

func New(ctx context.Context, opts Options) (*App, error) {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	dataDir, err := ResolveDataDir(opts.DataDir)
	if err != nil {
		return nil, err
	}
	cfg, err := LoadConfig(dataDir)
	if err != nil {
		return nil, err
	}
	if opts.ListenAddress != "" {
		if err := ValidateListenAddress(opts.ListenAddress); err != nil {
			return nil, fmt.Errorf("--listen: %w", err)
		}
		cfg.ListenAddress = opts.ListenAddress
	}

	// Bind early so a port conflict fails before any persistent service opens.
	ln, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", cfg.ListenAddress, err)
	}
	cleanup := func() {
		_ = ln.Close()
	}
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("resolve listener address: %w", err)
	}

	dbPath := filepath.Join(dataDir, databaseFileName)
	st, err := store.Open(ctx, dbPath)
	if err != nil {
		cleanup()
		return nil, err
	}
	cleanupStore := func() {
		_ = st.Close()
		cleanup()
	}
	settings, err := st.LoadSettings(ctx)
	if err != nil {
		cleanupStore()
		return nil, err
	}

	var secretStore secrets.Store
	var secretsStatus domain.SecretsStatus
	if opts.SessionOnlySecrets {
		secretStore = secrets.NewMemory()
		secretsStatus = domain.SecretsStatus{
			Mode:      domain.SecretsModeMemory,
			Available: true,
			Detail:    "Explicit session-only mode: secrets are held in memory for this process only.",
		}
	} else {
		osStore := secrets.NewOS("LiveCheck")
		secretStore = osStore
		if osStore.Available(ctx) {
			secretsStatus = domain.SecretsStatus{
				Mode:      domain.SecretsModeOS,
				Available: true,
				Detail:    "OS credential storage is available. P01 stores no credentials.",
			}
		} else {
			secretsStatus = domain.SecretsStatus{
				Mode:      domain.SecretsModeUnavailable,
				Available: false,
				Detail:    "OS credential storage is unavailable. Restart with --session-only-secrets for memory-only mode.",
			}
		}
	}

	baseCtx, baseCancel := context.WithCancel(context.Background())
	a := &App{
		cfg:           cfg,
		dataDir:       dataDir,
		now:           opts.Now,
		listener:      ln,
		store:         st,
		secrets:       secretStore,
		secretsState:  secretsStatus,
		stdout:        opts.Stdout,
		stderr:        opts.Stderr,
		noBrowser:     opts.NoBrowser,
		logger:        slog.New(slog.NewTextHandler(opts.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
		settings:      settings,
		stateRevision: 1,
		privateHub:    newHub(),
		publicHub:     newHub(),
		conns:         make(map[net.Conn]http.ConnState),
		baseCtx:       baseCtx,
		baseCancel:    baseCancel,
	}
	a.baseURL = "http://" + net.JoinHostPort(host, port)

	ui, err := httpui.New(httpui.Options{
		Backend: a,
		Host:    host,
		Port:    port,
		Mode:    domain.ModeLive,
		Version: Version,
		Assets:  web.FS,
		Now:     a.now,
	})
	if err != nil {
		baseCancel()
		cleanupStore()
		return nil, err
	}
	a.ui = ui
	a.controlURL = a.baseURL + "/control/#bootstrap=" + ui.BootstrapToken()
	a.httpServer = &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           ui.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
		BaseContext: func(net.Listener) context.Context {
			return baseCtx
		},
		ConnState: a.trackConnState,
	}
	return a, nil
}

func (a *App) ControlURL() string { return a.controlURL }
func (a *App) BaseURL() string    { return a.baseURL }
func (a *App) DataDir() string    { return a.dataDir }

func (a *App) Run(ctx context.Context) error {
	defer a.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.httpServer.Serve(a.listener)
	}()

	fmt.Fprintf(a.stdout, "CueMender %s\n", VersionString())
	fmt.Fprintf(a.stdout, "Listening on %s\n", a.baseURL)
	fmt.Fprintf(a.stdout, "Public overlay: %s/overlay/\n", a.baseURL)
	fmt.Fprintf(a.stdout, "Private control (single-use link, valid 5 minutes): %s\n", a.controlURL)
	fmt.Fprintln(a.stdout, "Twitch: not connected. OBS: not connected. Integrations arrive in P02.")
	a.logger.Info("listening", "address", a.baseURL)

	if !a.noBrowser {
		if err := openBrowser(a.controlURL); err != nil {
			fmt.Fprintf(a.stderr, "Could not open a browser: %v\nOpen the private control link above manually.\n", err)
		}
	}

	var runErr error
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("http server: %w", err)
		}
	}

	a.baseCancel()
	if a.ui != nil {
		a.ui.Close()
	}
	// Stop accepting new connections before closing idle/new sockets. A socket
	// accepted without a request can otherwise hold graceful shutdown until its
	// full deadline. Active handlers are ended by ui.Close above.
	if a.listener != nil {
		_ = a.listener.Close()
	}
	a.closeNewOrIdleConnections()
	a.httpServer.SetKeepAlivesEnabled(false)
	a.logger.Info("shutdown starting", "active_streams", a.ui.ActiveStreams())
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		a.logger.Warn("http shutdown incomplete", "error", err, "active_streams", a.ui.ActiveStreams())
		_ = a.httpServer.Close()
	}
	a.logger.Info("stopped")
	return runErr
}

func (a *App) Close() {
	a.closeOnce.Do(func() {
		a.baseCancel()
		if a.ui != nil {
			a.ui.Close()
		}
		if a.listener != nil {
			_ = a.listener.Close()
		}
		a.closeNewOrIdleConnections()
		if a.store != nil {
			_ = a.store.Close()
		}
	})
}

func (a *App) trackConnState(conn net.Conn, state http.ConnState) {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	if state == http.StateClosed || state == http.StateHijacked {
		delete(a.conns, conn)
		return
	}
	a.conns[conn] = state
}

// closeNewOrIdleConnections closes sockets that cannot be performing a live
// request. It prevents a just-accepted idle socket from holding shutdown open.
func (a *App) closeNewOrIdleConnections() {
	a.connMu.Lock()
	conns := make([]net.Conn, 0, len(a.conns))
	for conn, state := range a.conns {
		if state == http.StateNew || state == http.StateIdle {
			conns = append(conns, conn)
		}
	}
	a.connMu.Unlock()
	for _, conn := range conns {
		_ = conn.Close()
	}
}

func (a *App) PrivateSnapshot(context.Context) (domain.PrivateState, error) {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.privateLocked(), nil
}

func (a *App) PublicSnapshot(context.Context) (domain.PublicState, error) {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.publicLocked(), nil
}

func (a *App) SaveSettings(ctx context.Context, expectedRevision int64, exp domain.Expectations) (domain.Settings, error) {
	if exp.StreamTrack != nil {
		confirmed := a.now().UTC()
		exp.TrackConfirmedAt = &confirmed
	} else {
		exp.TrackConfirmedAt = nil
	}
	if err := exp.Validate(); err != nil {
		return domain.Settings{}, err
	}

	// Successful commits are serialized with cache installation and
	// publication. Without this, a writer paused after its SQL commit could
	// install an older revision after a newer writer, and its older snapshot
	// could be published after the newer one.
	a.saveMu.Lock()
	defer a.saveMu.Unlock()

	settings, err := a.store.SaveSettings(ctx, expectedRevision, exp)
	if err != nil {
		return domain.Settings{}, err
	}

	// stateMu is the publication ordering lock: cache update, revision
	// increment, both projections and both hub publications happen while a
	// subscriber cannot observe an intermediate combination. Hub sends are
	// bounded and non-blocking, so no network operation is held under stateMu.
	a.stateMu.Lock()
	a.settings = settings
	a.stateRevision++
	privateJSON, errPrivate := json.Marshal(a.privateLocked())
	publicJSON, errPublic := json.Marshal(a.publicLocked())
	if errPrivate == nil {
		a.privateHub.publish(privateJSON)
	}
	if errPublic == nil {
		a.publicHub.publish(publicJSON)
	}
	a.stateMu.Unlock()
	return settings, nil
}

func (a *App) SubscribePrivate() httpui.Subscription {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	data, err := json.Marshal(a.privateLocked())
	if err != nil {
		data = []byte(`{"mode":"live","state_revision":0,"monitoring_active":false}`)
	}
	return a.privateHub.subscribe(data)
}

func (a *App) SubscribePublic() httpui.Subscription {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	data, err := json.Marshal(a.publicLocked())
	if err != nil {
		data = []byte(`{"mode":"live","state_revision":0,"monitoring_active":false,"service_status":"disconnected","incidents":[]}`)
	}
	return a.publicHub.subscribe(data)
}

func (a *App) privateLocked() domain.PrivateState {
	return domain.PrivateState{
		Mode:             domain.ModeLive,
		StateRevision:    a.stateRevision,
		MonitoringActive: false,
		Settings:         a.settings,
		Setup: domain.SetupReadiness{
			MicrophoneConfigured:   a.settings.Microphone != nil,
			StreamTrackConfigured:  a.settings.StreamTrack != nil,
			VisualConfigured:       a.settings.Visual != nil,
			LiveOBSResolutionReady: false,
		},
		Secrets: a.secretsStatus(),
		Twitch: domain.IntegrationStatus{
			Status: domain.IntegrationNotConnected,
			Detail: "Twitch connection is not implemented in P01; it arrives in P02.",
		},
		OBS: domain.IntegrationStatus{
			Status: domain.IntegrationNotConnected,
			Detail: "OBS connection is not implemented in P01; it arrives in P02.",
		},
		Startup: domain.StartupInfo{
			ListenAddress:            a.cfg.ListenAddress,
			OBSAddress:               a.cfg.OBSAddress,
			TwitchClientIDConfigured: a.cfg.TwitchClientID != "",
		},
	}
}

func (a *App) publicLocked() domain.PublicState {
	return domain.PublicState{
		Mode:             domain.ModeLive,
		StateRevision:    a.stateRevision,
		MonitoringActive: false,
		ServiceStatus:    domain.ServiceDisconnected,
		Incidents:        []domain.PublicIncident{},
	}
}

func (a *App) secretsStatus() domain.SecretsStatus {
	return a.secretsState
}
