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
	"github.com/desanv01/cuemender/internal/incidents"
	"github.com/desanv01/cuemender/internal/obs"
	"github.com/desanv01/cuemender/internal/secrets"
	"github.com/desanv01/cuemender/internal/store"
	"github.com/desanv01/cuemender/internal/twitch"
	"github.com/desanv01/cuemender/web"
)

type Options struct {
	Mode               string
	DataDir            string
	ListenAddress      string
	NoBrowser          bool
	SessionOnlySecrets bool
	Stdout             io.Writer
	Stderr             io.Writer
	Now                func() time.Time
}

type osSecretStore interface {
	secrets.Store
	Available(context.Context) bool
}

// The private composition seam lets local fixtures exercise startup and the
// credential boundary. Public New always uses the real stores and endpoints.
type appDependencies struct {
	newOSStore       func() osSecretStore
	configureTwitch  func(*twitch.Options)
	configureOBS     func(*obs.Options)
	monitoringStatus func() incidents.Readiness
}

type App struct {
	cfg        Config
	fileConfig Config
	configMu   sync.Mutex
	dataDir    string
	mode       string
	now        func() time.Time

	listener                 net.Listener
	store                    *store.Store
	secrets                  secrets.Store
	secretsState             domain.SecretsStatus
	twitchMgr                *twitch.Manager
	obsMgr                   *obs.Manager
	coordinator              *incidents.Coordinator
	rehearsal                *rehearsalState
	monitoringStatusOverride func() incidents.Readiness

	ui         *httpui.Server
	httpServer *http.Server
	stdout     io.Writer
	stderr     io.Writer
	noBrowser  bool
	logger     *slog.Logger

	stateMu       sync.RWMutex
	saveMu        sync.Mutex
	integrationMu sync.Mutex
	settings      domain.Settings
	stateRevision uint64
	privateHub    *hub
	publicHub     *hub

	connMu sync.Mutex
	conns  map[net.Conn]http.ConnState

	baseCtx      context.Context
	baseCancel   context.CancelFunc
	closeOnce    sync.Once
	intakeCh     chan twitch.CommandEvent
	intakeCtx    context.Context
	intakeCancel context.CancelFunc
	intakeWG     sync.WaitGroup

	baseURL    string
	controlURL string
}

func New(ctx context.Context, opts Options) (*App, error) {
	return newWithDependencies(ctx, opts, appDependencies{})
}

func newWithDependencies(ctx context.Context, opts Options, deps appDependencies) (*App, error) {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	mode := opts.Mode
	if mode == "" {
		mode = domain.ModeLive
	}
	if mode != domain.ModeLive && mode != domain.ModeRehearsal {
		return nil, domain.ErrModeConflict
	}
	baseDir, err := ResolveDataDir(opts.DataDir)
	if err != nil {
		return nil, err
	}
	dataDir := baseDir
	if mode == domain.ModeRehearsal {
		dataDir, err = ResolveDataDir(filepath.Join(baseDir, "rehearsal"))
		if err != nil {
			return nil, err
		}
	}
	cfg, err := LoadConfig(dataDir)
	if err != nil {
		return nil, err
	}
	fileConfig := cfg
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
	st, err := store.OpenMode(ctx, dbPath, mode)
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
	if opts.SessionOnlySecrets || mode == domain.ModeRehearsal {
		secretStore = secrets.NewMemory()
		secretsStatus = domain.SecretsStatus{
			Mode:      domain.SecretsModeMemory,
			Available: true,
			Detail:    "Memory-only secrets for this process.",
		}
	} else {
		newOSStore := deps.newOSStore
		if newOSStore == nil {
			newOSStore = func() osSecretStore { return secrets.NewOS("LiveCheck") }
		}
		osStore := newOSStore()
		secretStore = osStore
		if osStore.Available(ctx) {
			secretsStatus = domain.SecretsStatus{
				Mode:      domain.SecretsModeOS,
				Available: true,
				Detail:    "OS credential storage is available.",
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
		fileConfig:    fileConfig,
		dataDir:       dataDir,
		mode:          mode,
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
	intakeCtx, intakeCancel := context.WithCancel(baseCtx)
	a.intakeCtx = intakeCtx
	a.intakeCancel = intakeCancel
	a.intakeCh = make(chan twitch.CommandEvent, 32)
	a.intakeWG.Add(1)
	go func() {
		defer a.intakeWG.Done()
		for {
			select {
			case <-intakeCtx.Done():
				return
			case event := <-a.intakeCh:
				a.processTwitchCommand(event)
			}
		}
	}()
	a.baseURL = "http://" + net.JoinHostPort(host, port)
	if mode == domain.ModeRehearsal {
		a.rehearsal = newRehearsalState(settings)
	} else {
		twitchOptions := twitch.Options{
			Context:               baseCtx,
			OAuth:                 twitch.NewOAuth(&http.Client{Timeout: 15 * time.Second}, twitch.DefaultEndpoints(), a.now),
			Store:                 twitch.NewCredentialStore(secretStore),
			Dialer:                twitch.CoderDialer{HTTP: &http.Client{}},
			Endpoints:             twitch.DefaultEndpoints(),
			ClientID:              fileConfig.TwitchClientID,
			StorageMode:           secretsStatus.Mode,
			Now:                   a.now,
			OnUpdate:              a.onIntegrationUpdate,
			OnCommand:             a.onTwitchCommand,
			SuppressStaticReplies: true,
			RefreshPeriod:         time.Hour,
		}
		if deps.configureTwitch != nil {
			deps.configureTwitch(&twitchOptions)
		}
		a.twitchMgr = twitch.NewManager(twitchOptions)
		obsOptions := obs.Options{
			Context:  baseCtx,
			Dialer:   obs.GoobsDialer{},
			Secrets:  secretStore,
			Now:      a.now,
			OnUpdate: a.onIntegrationUpdate,
		}
		if deps.configureOBS != nil {
			deps.configureOBS(&obsOptions)
		}
		a.obsMgr = obs.NewManager(obsOptions)
		a.obsMgr.SetSettings(settings.Revision, settings.Expectations)
	}
	a.monitoringStatusOverride = deps.monitoringStatus
	coordinatorOptions := incidents.Options{
		Store:    st,
		Settings: a.currentSettings,
		Status:   a.monitoringReadiness,
		OnChange: a.onIncidentUpdate,
		Now:      a.now,
		Mode:     mode,
	}
	if a.rehearsal != nil {
		coordinatorOptions.Read = a.rehearsal.Read
		coordinatorOptions.Repair = a.rehearsal.Repair
		coordinatorOptions.ReadTrack = a.rehearsal.ReadTrack
		coordinatorOptions.Reply = func(string) bool { return true }
	} else {
		coordinatorOptions.Repair = a.obsMgr.RepairTrack
		coordinatorOptions.ReadTrack = a.obsMgr.ReadTrackState
		coordinatorOptions.Read = func(ctx context.Context, snapshot domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
			return a.obsMgr.DiagnosticRead(ctx, snapshot)
		}
		coordinatorOptions.Reply = a.twitchMgr.EnqueueReply
	}
	coordinator, err := incidents.NewChecked(coordinatorOptions)
	if err != nil {
		a.Close()
		return nil, err
	}
	a.coordinator = coordinator

	ui, err := httpui.New(httpui.Options{
		Backend: a,
		Host:    host,
		Port:    port,
		Mode:    mode,
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
func (a *App) Mode() string       { return a.mode }

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
	fmt.Fprintln(a.stdout, "Twitch: not connected. OBS: not connected. Use the private control page to connect.")
	a.logger.Info("listening", "address", a.baseURL)
	if a.twitchMgr != nil {
		a.twitchMgr.StartSaved()
		a.twitchMgr.StartRefreshLoop()
	}

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
		if a.intakeCancel != nil {
			a.intakeCancel()
		}
		a.intakeWG.Wait()
		if a.ui != nil {
			a.ui.Close()
		}
		if a.coordinator != nil {
			a.coordinator.Close()
		}
		if a.twitchMgr != nil {
			a.twitchMgr.Close()
		}
		if a.obsMgr != nil {
			a.obsMgr.Close()
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

	var settings domain.Settings
	commit := func() error {
		var err error
		settings, err = a.store.SaveSettings(ctx, expectedRevision, exp)
		if err != nil {
			return err
		}
		if a.obsMgr != nil {
			a.obsMgr.SetSettings(settings.Revision, settings.Expectations)
		}

		return nil
	}
	install := func() {
		a.stateMu.Lock()
		defer a.stateMu.Unlock()
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
	}

	var err error
	if a.coordinator != nil {
		err = a.coordinator.WithSettingsChange(ctx, commit, install)
	} else {
		err = commit()
		if err == nil {
			install()
		}
	}
	if err != nil {
		return domain.Settings{}, err
	}
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
	var twitchState domain.TwitchConnection
	var obsState domain.OBSConnection
	var discovery *domain.OBSDiscovery
	var observation *domain.OBSObservation
	resolutionReady := false
	if a.rehearsal != nil {
		now := a.now().UTC()
		twitchState = domain.TwitchConnection{Status: domain.ConnectionConnected, ConnectionGeneration: 1, Login: "rehearsal"}
		obsState = domain.OBSConnection{Status: domain.ConnectionConnected, Generation: a.rehearsal.Generation(), ConfigRevision: a.settings.Revision, SceneCollection: "Rehearsal", ObservedAt: &now, Broadcasting: boolPtr(true)}
		discovery, observation, _ = a.rehearsal.Read(context.Background(), a.settings)
		resolutionReady = true
	} else {
		twitchState = a.twitchMgr.Snapshot()
		obsState, discovery, observation, resolutionReady = a.obsMgr.SnapshotBundle(a.settings)
	}
	var session *domain.MonitoringSessionInfo
	var incidentsList []domain.Incident
	var proposals []domain.Proposal
	var actionRecords []domain.Action
	if a.coordinator != nil {
		session, incidentsList = a.coordinator.Snapshot()
		proposals, actionRecords = a.coordinator.RepairSnapshot()
	}
	monitoringActive := session != nil && session.Active
	return domain.PrivateState{
		Proposals: proposals, Actions: actionRecords,
		Mode:              a.mode,
		StateRevision:     a.stateRevision,
		MonitoringActive:  monitoringActive,
		Settings:          a.settings,
		MonitoringSession: session,
		Incidents:         incidentsList,
		Setup: domain.SetupReadiness{
			MicrophoneConfigured:   a.settings.Microphone != nil,
			StreamTrackConfigured:  a.settings.StreamTrack != nil,
			VisualConfigured:       a.settings.Visual != nil,
			TwitchAuthorized:       twitchState.Status == domain.ConnectionConnected,
			OBSConnected:           obsState.Status == domain.ConnectionConnected,
			OBSDiscoveryReady:      discovery != nil,
			LiveOBSResolutionReady: obsState.Status == domain.ConnectionConnected && !obsState.Stale && resolutionReady,
		},
		Secrets:        a.secretsStatus(),
		Twitch:         twitchState,
		OBS:            obsState,
		OBSDiscovery:   discovery,
		OBSObservation: observation,
		Startup: domain.StartupInfo{
			ListenAddress:            a.cfg.ListenAddress,
			OBSAddress:               a.cfg.OBSAddress,
			TwitchClientIDConfigured: a.cfg.TwitchClientID != "",
		},
	}
}

func (a *App) publicLocked() domain.PublicState {
	active := false
	var publicIncidents []domain.PublicIncident
	jobs := 0
	if a.coordinator != nil {
		active, publicIncidents, jobs = a.coordinator.PublicSnapshot()
	}
	status := domain.ServiceDisconnected
	if active {
		if a.rehearsal != nil {
			status = "ready"
			if jobs > 0 {
				status = "checking"
			}
		} else {
			tw := a.twitchMgr.Snapshot()
			obs, _, _, _ := a.obsMgr.SnapshotBundle(a.settings)
			if tw.Status == domain.ConnectionConnected && obs.Status == domain.ConnectionConnected && !obs.Stale {
				status = "ready"
				if jobs > 0 {
					status = "checking"
				}
			}
		}
	}
	return domain.PublicState{
		Mode:             a.mode,
		StateRevision:    a.stateRevision,
		MonitoringActive: active,
		ServiceStatus:    status,
		Incidents:        publicIncidents,
		DemoLabel:        rehearsalLabel(a.mode),
	}
}

func rehearsalLabel(mode string) string {
	if mode == domain.ModeRehearsal {
		return "REHEARSAL — simulated data"
	}
	return ""
}

func (a *App) secretsStatus() domain.SecretsStatus {
	return a.secretsState
}
