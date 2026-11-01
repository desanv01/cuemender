package obs

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/secrets"
)

var (
	ErrBusy                   = errors.New("obs lifecycle is busy")
	ErrUnavailable            = errors.New("obs discovery is unavailable")
	ErrCredentialsUnavailable = errors.New("obs credential storage is unavailable")
)

type Options struct {
	Context      context.Context
	Dialer       Dialer
	Secrets      secrets.Store
	Now          func() time.Time
	OnUpdate     func()
	PollInterval time.Duration
	StaleAfter   time.Duration
}

type Manager struct {
	ctx          context.Context
	cancel       context.CancelFunc
	dialer       Dialer
	secrets      secrets.Store
	now          func() time.Time
	onUpdate     func()
	pollInterval time.Duration
	staleAfter   time.Duration

	lifecycleMu       sync.Mutex
	mu                sync.Mutex
	gen               uint64
	conn              domain.OBSConnection
	endpoint          string
	backend           Backend
	discovery         *domain.OBSDiscovery
	observation       *domain.OBSObservation
	settings          domain.Expectations
	configRevision    int64
	authPassword      string
	workerCancel      context.CancelFunc
	workerCtx         context.Context
	operations        chan struct{}
	needsRevalidation bool
	workerWG          sync.WaitGroup
	closed            bool
}

func NewManager(opts Options) *Manager {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	managerCtx, cancel := context.WithCancel(ctx)
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 2 * time.Second
	}
	if opts.StaleAfter <= 0 {
		opts.StaleAfter = 5 * time.Second
	}
	return &Manager{
		ctx:          managerCtx,
		operations:   make(chan struct{}, 1),
		cancel:       cancel,
		dialer:       opts.Dialer,
		secrets:      opts.Secrets,
		now:          opts.Now,
		onUpdate:     opts.OnUpdate,
		pollInterval: opts.PollInterval,
		staleAfter:   opts.StaleAfter,
		conn:         domain.OBSConnection{Status: domain.ConnectionNotConnected},
	}
}

func (m *Manager) Connect(address string, password *string) error {
	if err := ValidateEndpoint(address); err != nil {
		return err
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrBusy
	}
	if m.conn.Status == domain.ConnectionConnecting || m.conn.Status == domain.ConnectionConnected || m.conn.Status == domain.ConnectionReconnecting {
		m.mu.Unlock()
		return ErrBusy
	}
	m.mu.Unlock()

	var resolvedPassword string
	if password != nil {
		resolvedPassword = *password
		if m.secrets != nil {
			key := passwordKey(address)
			var err error
			if resolvedPassword == "" {
				err = m.secrets.Delete(m.ctx, key)
				if secrets.IsNotFound(err) {
					err = nil
				}
			} else {
				err = m.secrets.Set(m.ctx, key, resolvedPassword)
			}
			if err != nil {
				return ErrCredentialsUnavailable
			}
		}
	} else if m.secrets != nil {
		// Password omission always means "reuse the credential saved for this
		// exact endpoint", including in a fresh manager after restart. A
		// different endpoint has a different key and cannot inherit it.
		value, err := m.secrets.Get(m.ctx, passwordKey(address))
		if err == nil {
			resolvedPassword = value
		} else if !secrets.IsNotFound(err) {
			return ErrCredentialsUnavailable
		}
	}
	m.mu.Lock()
	if m.workerCancel != nil {
		m.workerCancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.workerCancel = cancel
	m.workerCtx = ctx
	m.gen++
	gen := m.gen
	m.endpoint = address
	m.authPassword = resolvedPassword
	m.backend = nil
	m.discovery = nil
	m.observation = nil
	m.conn = domain.OBSConnection{
		Status:         domain.ConnectionConnecting,
		DetailCode:     "connecting",
		Detail:         "Connecting to the local OBS WebSocket server.",
		Address:        address,
		Generation:     gen,
		ConfigRevision: m.configRevision,
	}
	m.workerWG.Add(1)
	m.mu.Unlock()
	m.notify()
	go func() {
		defer m.workerWG.Done()
		m.connectFlow(ctx, gen, address, resolvedPassword)
	}()
	return nil
}

func (m *Manager) connectFlow(ctx context.Context, gen uint64, address, password string) {
	var backend Backend
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if !m.current(gen) {
			return
		}
		backend, err = m.dialer.Connect(ctx, address, password)
		if err == nil {
			break
		}
		if ctx.Err() != nil || !m.current(gen) {
			return
		}
		m.setState(gen, domain.ConnectionReconnecting, "connection_retry", "OBS WebSocket is retrying.")
		if !m.sleep(ctx, time.Duration(attempt+1)*500*time.Millisecond) {
			return
		}
	}
	if err != nil {
		if ctx.Err() == nil {
			m.fail(gen, "connection_failed", "OBS WebSocket rejected or could not be reached.")
		}
		return
	}
	if !m.current(gen) {
		_ = backend.Close()
		return
	}
	m.mu.Lock()
	if m.gen != gen || m.closed {
		m.mu.Unlock()
		_ = backend.Close()
		return
	}
	// Publish the candidate transport before the first SDK read so
	// Disconnect/Close can close it while Version/discovery is pending.
	m.backend = backend
	m.mu.Unlock()
	if _, err := backend.Version(ctx); err != nil {
		m.retireCandidate(gen, backend)
		m.fail(gen, "version_read_failed", "OBS connected, but the version read failed.")
		return
	}
	discovery, observation, err := m.readCycle(ctx, gen, backend)
	if err != nil {
		m.retireCandidate(gen, backend)
		if ctx.Err() == nil {
			m.fail(gen, "discovery_failed", "OBS connected, but the initial read failed.")
		}
		return
	}
	m.mu.Lock()
	if m.gen != gen || m.closed {
		m.mu.Unlock()
		_ = backend.Close()
		return
	}
	m.backend = backend
	if discovery.ConfigRevision == m.configRevision && observation.ConfigRevision == m.configRevision {
		m.discovery = discovery
		m.observation = observation
	} else {
		m.discovery = nil
		m.observation = nil
	}
	m.conn.Status = domain.ConnectionConnected
	m.conn.DetailCode = "connected"
	m.conn.Detail = "OBS WebSocket is connected. Read-only discovery is available."
	m.conn.Address = address
	m.conn.Generation = gen
	m.conn.ConfigRevision = m.configRevision
	m.conn.SceneCollection = discovery.SceneCollection
	m.mu.Unlock()
	m.notify()
	m.pollLoop(ctx, gen, backend)
}

func (m *Manager) retireCandidate(gen uint64, backend Backend) {
	m.mu.Lock()
	if m.backend == backend {
		m.backend = nil
	}
	m.mu.Unlock()
	_ = backend.Close()
}

func (m *Manager) pollLoop(ctx context.Context, gen uint64, backend Backend) {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !m.current(gen) {
				return
			}
			select {
			case m.operations <- struct{}{}:
			default:
				continue
			}
			discovery, observation, err := m.readCycle(ctx, gen, backend)
			<-m.operations
			if err != nil {
				m.handleTransportLoss(ctx, gen, backend)
				return
			}
			m.mu.Lock()
			if m.gen != gen || m.closed {
				m.mu.Unlock()
				return
			}
			if discovery.ConfigRevision == m.configRevision && observation.ConfigRevision == m.configRevision {
				m.discovery = discovery
				m.observation = observation
			} else {
				m.discovery = nil
				m.observation = nil
			}
			if m.conn.Status == domain.ConnectionReconnecting {
				m.conn.Status = domain.ConnectionConnected
				m.conn.DetailCode = "connected"
				m.conn.Detail = "OBS WebSocket is connected. Read-only discovery is available."
			}
			m.conn.ConfigRevision = m.configRevision
			m.conn.SceneCollection = discovery.SceneCollection
			m.mu.Unlock()
			m.notify()
		}
	}
}

func (m *Manager) handleTransportLoss(ctx context.Context, gen uint64, backend Backend) {
	m.mu.Lock()
	if m.gen != gen || m.backend != backend || m.closed {
		m.mu.Unlock()
		return
	}
	m.gen++
	m.needsRevalidation = true
	nextGen := m.gen
	address := m.endpoint
	password := m.authPassword
	m.backend = nil
	m.discovery = nil
	m.observation = nil
	m.conn.Status = domain.ConnectionReconnecting
	m.conn.DetailCode = "connection_lost"
	m.conn.Detail = "OBS WebSocket was lost. Reconnecting."
	m.conn.Generation = nextGen
	m.conn.Stale = true
	m.conn.Broadcasting = nil
	m.conn.ObservedAt = nil
	// Registration is atomic with lifecycle invalidation. The parent poll
	// worker remains owned until this replacement has been registered.
	m.workerWG.Add(1)
	m.mu.Unlock()
	_ = backend.Close()
	go func() {
		defer m.workerWG.Done()
		m.reconnectFlow(ctx, nextGen, address, password)
	}()
	m.notify()
}

func (m *Manager) reconnectFlow(ctx context.Context, gen uint64, address, password string) {
	backoff := 500 * time.Millisecond
	for {
		if ctx.Err() != nil || !m.current(gen) {
			return
		}
		backend, err := m.dialer.Connect(ctx, address, password)
		if err == nil {
			m.mu.Lock()
			if m.gen != gen || m.closed {
				m.mu.Unlock()
				_ = backend.Close()
				return
			}
			m.backend = backend
			m.mu.Unlock()
			if _, verr := backend.Version(ctx); verr == nil {
				discovery, observation, derr := m.readCycle(ctx, gen, backend)
				if derr == nil && m.current(gen) {
					m.mu.Lock()
					if m.gen != gen || m.closed {
						m.mu.Unlock()
						_ = backend.Close()
						return
					}
					m.backend = backend
					if discovery.ConfigRevision == m.configRevision && observation.ConfigRevision == m.configRevision {
						m.discovery = discovery
						m.observation = observation
					} else {
						m.discovery = nil
						m.observation = nil
					}
					m.conn.Status = domain.ConnectionConnected
					m.conn.DetailCode = "connected"
					m.conn.Detail = "OBS WebSocket is connected. Read-only discovery is available."
					m.conn.Stale = false
					m.conn.SceneCollection = discovery.SceneCollection
					m.mu.Unlock()
					m.notify()
					m.pollLoop(ctx, gen, backend)
					return
				}
			}
			m.retireCandidate(gen, backend)
		}
		if ctx.Err() != nil || !m.current(gen) {
			return
		}
		m.setState(gen, domain.ConnectionReconnecting, "connection_lost", "OBS WebSocket was lost. Reconnecting.")
		if !m.sleep(ctx, backoff) {
			return
		}
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
}

func (m *Manager) readCycle(ctx context.Context, gen uint64, backend Backend) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
	m.mu.Lock()
	revision, settings := m.configRevision, cloneExpectations(m.settings)
	m.mu.Unlock()
	discovery, err := m.readDiscoveryAt(ctx, gen, backend, revision)
	if err != nil {
		return nil, nil, err
	}
	observation, err := m.readObservation(ctx, gen, backend, discovery, settings)
	return discovery, observation, err
}

func (m *Manager) readDiscovery(ctx context.Context, gen uint64, backend Backend) (*domain.OBSDiscovery, error) {
	m.mu.Lock()
	revision := m.configRevision
	m.mu.Unlock()
	return m.readDiscoveryAt(ctx, gen, backend, revision)
}

func (m *Manager) readDiscoveryAt(ctx context.Context, gen uint64, backend Backend, revision int64) (*domain.OBSDiscovery, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	collection := ""
	collectionKnown := false
	collectionReader, hasCollection := backend.(interface {
		SceneCollection(context.Context) (string, error)
	})
	if hasCollection {
		if value, err := collectionReader.SceneCollection(ctx); err == nil {
			collection = value
			collectionKnown = value != ""
		}
	}
	inputs, err := backend.Inputs(ctx)
	if err != nil {
		return nil, err
	}
	scenes, err := backend.Scenes(ctx)
	if err != nil {
		return nil, err
	}
	if len(inputs) > 500 {
		inputs = inputs[:500]
	}
	if len(scenes) > 100 {
		scenes = scenes[:100]
	}
	for i := range scenes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items, err := backend.SceneItems(ctx, scenes[i].UUID)
		if err != nil {
			return nil, err
		}
		if len(items) > 500 {
			items = items[:500]
		}
		scenes[i].Items = items
	}
	if hasCollection {
		if value, err := collectionReader.SceneCollection(ctx); err == nil {
			// A collection switch during the identity sequence makes the
			// combined snapshot ambiguous; do not silently relabel it.
			if collectionKnown && value != "" && value == collection {
				collectionKnown = true
			} else {
				collectionKnown = false
				collection = ""
			}
		} else {
			collectionKnown = false
			collection = ""
		}
	}
	m.mu.Lock()
	if m.gen == gen && m.configRevision == revision && (!collectionKnown || (m.discovery != nil && m.discovery.SceneCollection != collection)) {
		m.needsRevalidation = true
	}
	m.mu.Unlock()
	return &domain.OBSDiscovery{
		Generation:      gen,
		ConfigRevision:  revision,
		SceneCollection: collection,
		CollectionKnown: collectionKnown,
		CapturedAt:      m.now().UTC(),
		Inputs:          inputs,
		Scenes:          scenes,
	}, nil
}

var errTransportLost = errors.New("OBS transport lost")

func (m *Manager) readObservation(ctx context.Context, gen uint64, backend Backend, discovery *domain.OBSDiscovery, settings domain.Expectations) (*domain.OBSObservation, error) {
	observation := &domain.OBSObservation{
		Generation:      gen,
		ConfigRevision:  discovery.ConfigRevision,
		SceneCollection: discovery.SceneCollection,
		ObservedAt:      m.now().UTC(),
	}
	attempts, successes := 0, 0
	record := func(name string, err error) {
		attempts++
		if err != nil {
			observation.Partial = append(observation.Partial, name)
			return
		}
		successes++
	}

	if settings.Microphone != nil {
		mic := *settings.Microphone
		observation.Microphone = &mic
		mute, err := backend.InputMute(ctx, mic)
		record("mute", err)
		if err == nil {
			observation.Mute = mute
		}
		tracks, err := backend.InputAudioTracks(ctx, mic)
		record("tracks", err)
		if err == nil && completeTracks(tracks) {
			observation.Tracks = tracks
		} else if err == nil {
			observation.Partial = append(observation.Partial, "tracks")
		}
	}

	sceneName, sceneUUID, err := backend.ProgramScene(ctx)
	record("program_scene", err)
	if err == nil {
		observation.ProgramScene = sceneName
		observation.ProgramSceneUUID = sceneUUID
	}

	if settings.Visual != nil {
		item, scene := findVisualItem(discovery, *settings.Visual)
		if scene == nil || item == nil {
			observation.Partial = append(observation.Partial, "visual")
		} else {
			enabled, err := backend.SceneItemEnabled(ctx, scene.UUID, item.ID)
			record("visual", err)
			if err == nil {
				observation.VisualItemEnabled = enabled
			}
		}
	}

	streaming, err := backend.StreamStatus(ctx)
	record("streaming", err)
	if err == nil {
		observation.Streaming = streaming
	}
	if reader, ok := backend.(interface {
		SceneCollection(context.Context) (string, error)
	}); ok {
		collection, err := reader.SceneCollection(ctx)
		if err != nil || !discovery.CollectionKnown || collection == "" || collection != discovery.SceneCollection {
			discovery.CollectionKnown = false
			discovery.SceneCollection = ""
			observation.SceneCollection = ""
			observation.ProgramScene, observation.ProgramSceneUUID = "", ""
			observation.Mute, observation.Tracks, observation.VisualItemEnabled = nil, nil, nil
			observation.Partial = append(observation.Partial, "context")
			m.mu.Lock()
			if m.gen == gen && m.configRevision == discovery.ConfigRevision {
				m.needsRevalidation = true
			}
			m.mu.Unlock()
		}
	}
	if attempts > 0 && successes == 0 {
		return nil, errTransportLost
	}
	return observation, nil
}

func findVisualItem(discovery *domain.OBSDiscovery, visual domain.VisualExpectation) (*domain.OBSSceneItem, *domain.OBSScene) {
	if discovery == nil {
		return nil, nil
	}
	var matchedItem *domain.OBSSceneItem
	var matchedScene *domain.OBSScene
	matches := 0
	for i := range discovery.Scenes {
		scene := &discovery.Scenes[i]
		if visual.Scene != "" && scene.Name != visual.Scene {
			continue
		}
		if visual.SceneUUID != "" && scene.UUID != visual.SceneUUID {
			continue
		}
		for j := range scene.Items {
			item := &scene.Items[j]
			if visual.SceneItemID != "" && fmt.Sprint(item.ID) != visual.SceneItemID {
				continue
			}
			if visual.Source.Name != "" && item.SourceName != visual.Source.Name {
				continue
			}
			if visual.Source.UUID != "" && item.SourceUUID != visual.Source.UUID {
				continue
			}
			matchedItem, matchedScene = item, scene
			matches++
		}
	}
	if matches == 1 {
		return matchedItem, matchedScene
	}
	return nil, nil
}

func completeTracks(tracks map[string]bool) bool {
	if len(tracks) == 0 {
		return false
	}
	for i := 1; i <= 6; i++ {
		if _, ok := tracks[fmt.Sprintf("track%d", i)]; !ok {
			return false
		}
	}
	return true
}

func (m *Manager) SetSettings(revision int64, settings domain.Expectations) {
	m.mu.Lock()
	changed := m.configRevision != revision || !expectationsEqual(m.settings, settings)
	m.configRevision = revision
	m.settings = cloneExpectations(settings)
	if changed {
		m.observation = nil
		if m.discovery != nil && m.discovery.CollectionKnown && m.conn.Status == domain.ConnectionConnected {
			m.needsRevalidation = false
		} else if hasNameOnly(settings) {
			m.needsRevalidation = true
		}
	}
	m.mu.Unlock()
}

func cloneExpectations(in domain.Expectations) domain.Expectations {
	out := in
	if in.Microphone != nil {
		value := *in.Microphone
		out.Microphone = &value
	}
	if in.Visual != nil {
		value := *in.Visual
		out.Visual = &value
	}
	if in.StreamTrack != nil {
		value := *in.StreamTrack
		out.StreamTrack = &value
	}
	if in.TrackConfirmedAt != nil {
		value := *in.TrackConfirmedAt
		out.TrackConfirmedAt = &value
	}
	return out
}

func hasNameOnly(settings domain.Expectations) bool {
	return (settings.Microphone != nil && settings.Microphone.UUID == "") ||
		(settings.Visual != nil && (settings.Visual.Source.UUID == "" || settings.Visual.SceneUUID == ""))
}

func expectationsEqual(a, b domain.Expectations) bool {
	return reflect.DeepEqual(a, b)
}

func (m *Manager) SetExpectations(settings domain.Expectations) {
	m.SetSettings(0, settings)
}

func (m *Manager) Snapshot() domain.OBSConnection {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn := m.conn
	if m.observation != nil && m.observation.ConfigRevision == m.configRevision {
		when := m.observation.ObservedAt
		conn.ObservedAt = &when
		conn.Broadcasting = copyBool(m.observation.Streaming)
		conn.Stale = conn.Status != domain.ConnectionConnected || m.now().Sub(when) > m.staleAfter
	} else {
		conn.Stale = true
	}
	conn.ConfigRevision = m.configRevision
	return conn
}

// SnapshotBundle returns connection, discovery, observation and resolution from
// one lock boundary so the app cannot mix generations or settings revisions.
func (m *Manager) SnapshotBundle(settings domain.Settings) (domain.OBSConnection, *domain.OBSDiscovery, *domain.OBSObservation, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn := m.conn
	conn.ConfigRevision = m.configRevision
	if m.observation != nil && m.observation.ConfigRevision == m.configRevision && settings.Revision == m.configRevision {
		when := m.observation.ObservedAt
		conn.ObservedAt = &when
		conn.Broadcasting = copyBool(m.observation.Streaming)
		conn.Stale = conn.Status != domain.ConnectionConnected || m.now().Sub(when) > m.staleAfter
	} else {
		conn.Stale = true
	}
	discovery := copyDiscovery(m.discovery)
	observation := copyObservation(m.observation)
	if observation != nil && (observation.ConfigRevision != m.configRevision || settings.Revision != m.configRevision) {
		observation = nil
	}
	ready := conn.Status == domain.ConnectionConnected && !conn.Stale &&
		resolutionReady(m.discovery, settings.Expectations) &&
		discovery != nil && discovery.ConfigRevision == settings.Revision && discovery.Generation == m.gen &&
		discovery.CollectionKnown && discovery.SceneCollection != "" &&
		(!m.needsRevalidation || !hasNameOnly(settings.Expectations)) &&
		settings.Revision == m.configRevision &&
		observation != nil && observation.ConfigRevision == settings.Revision &&
		observation.Generation == m.gen &&
		(discovery == nil || !observation.ObservedAt.Before(discovery.CapturedAt))
	if discovery != nil && observation != nil && discovery.SceneCollection != "" && observation.SceneCollection != discovery.SceneCollection {
		ready = false
	}
	return conn, discovery, observation, ready
}

func (m *Manager) Discovery() *domain.OBSDiscovery {
	m.mu.Lock()
	defer m.mu.Unlock()
	return copyDiscovery(m.discovery)
}

func copyDiscovery(in *domain.OBSDiscovery) *domain.OBSDiscovery {
	if in == nil {
		return nil
	}
	copyDiscovery := *in
	copyDiscovery.Inputs = append([]domain.OBSInput(nil), in.Inputs...)
	copyDiscovery.Scenes = make([]domain.OBSScene, len(in.Scenes))
	for i := range in.Scenes {
		copyDiscovery.Scenes[i] = in.Scenes[i]
		copyDiscovery.Scenes[i].Items = append([]domain.OBSSceneItem(nil), in.Scenes[i].Items...)
	}
	return &copyDiscovery
}

func (m *Manager) Observation() *domain.OBSObservation {
	m.mu.Lock()
	defer m.mu.Unlock()
	return copyObservation(m.observation)
}

func copyObservation(in *domain.OBSObservation) *domain.OBSObservation {
	if in == nil {
		return nil
	}
	copyObs := *in
	if in.Microphone != nil {
		value := *in.Microphone
		copyObs.Microphone = &value
	}
	if in.Mute != nil {
		value := *in.Mute
		copyObs.Mute = &value
	}
	if in.Streaming != nil {
		value := *in.Streaming
		copyObs.Streaming = &value
	}
	if in.VisualItemEnabled != nil {
		value := *in.VisualItemEnabled
		copyObs.VisualItemEnabled = &value
	}
	if in.Tracks != nil {
		copyObs.Tracks = make(map[string]bool, len(in.Tracks))
		for key, value := range in.Tracks {
			copyObs.Tracks[key] = value
		}
	}
	copyObs.Partial = append([]string(nil), in.Partial...)
	return &copyObs
}

func copyBool(in *bool) *bool {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

// ResolutionReady checks configured references against current discovery. A
// same-name replacement with a mismatched UUID is not considered ready.
func (m *Manager) ResolutionReady(settings domain.Expectations) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return (!m.needsRevalidation || !hasNameOnly(settings)) && resolutionReady(m.discovery, settings)
}

func resolutionReady(discovery *domain.OBSDiscovery, settings domain.Expectations) bool {
	if discovery == nil || !discovery.CollectionKnown || discovery.SceneCollection == "" {
		return false
	}
	if settings.Microphone != nil {
		if settings.Microphone.Context != "" &&
			(!discovery.CollectionKnown || settings.Microphone.Context != discovery.SceneCollection) {
			return false
		}
		if !inputResolves(discovery.Inputs, *settings.Microphone) {
			return false
		}
	}
	if settings.Visual != nil {
		if settings.Visual.Source.Context != "" &&
			(!discovery.CollectionKnown || settings.Visual.Source.Context != discovery.SceneCollection) {
			return false
		}
		if _, scene := findVisualItem(discovery, *settings.Visual); scene == nil {
			return false
		}
	}
	return true
}

func inputResolves(inputs []domain.OBSInput, ref domain.InputRef) bool {
	matches := 0
	for _, input := range inputs {
		if ref.UUID != "" {
			if input.UUID == ref.UUID && input.Name == ref.Name {
				matches++
			}
			continue
		}
		if input.Name == ref.Name {
			matches++
		}
	}
	return matches == 1
}

// DiagnosticRead performs a fresh, read-only snapshot for P03 under the same
// admission/cancellation ownership as RefreshDiscovery. It returns immutable
// revision/generation evidence and never installs cached state.
func (m *Manager) DiagnosticRead(ctx context.Context, settings domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
	m.mu.Lock()
	backend, gen, revision := m.backend, m.gen, m.configRevision
	if backend == nil || m.closed || m.conn.Status != domain.ConnectionConnected {
		m.mu.Unlock()
		return nil, nil, ErrUnavailable
	}
	if settings.Revision != revision {
		m.mu.Unlock()
		return nil, nil, ErrUnavailable
	}
	select {
	case m.operations <- struct{}{}:
	default:
		m.mu.Unlock()
		return nil, nil, ErrBusy
	}
	m.workerWG.Add(1)
	workerCtx := m.workerCtx
	if workerCtx == nil {
		workerCtx = m.ctx
	}
	m.mu.Unlock()
	defer m.workerWG.Done()
	defer func() { <-m.operations }()
	readCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(workerCtx, cancel)
	defer func() { stop(); cancel() }()
	readCtx, deadline := context.WithTimeout(readCtx, 5*time.Second)
	defer deadline()
	discovery, err := m.readDiscoveryAt(readCtx, gen, backend, revision)
	if err != nil {
		m.handleTransportLoss(workerCtx, gen, backend)
		return nil, nil, ErrUnavailable
	}
	observation, err := m.readObservation(readCtx, gen, backend, discovery, settings.Expectations)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	m.mu.Lock()
	valid := m.gen == gen && m.backend == backend && !m.closed &&
		m.configRevision == revision && readCtx.Err() == nil
	m.mu.Unlock()
	if !valid || discovery.ConfigRevision != revision || discovery.Generation != gen ||
		observation.ConfigRevision != revision || observation.Generation != gen {
		return nil, nil, ErrUnavailable
	}
	return discovery, observation, nil
}

// RefreshDiscovery performs a bounded fresh read using the active transport.
// It never returns a forever-cached snapshot as refreshed.
func (m *Manager) RefreshDiscovery(ctx context.Context) (*domain.OBSDiscovery, error) {
	m.mu.Lock()
	backend, gen, revision := m.backend, m.gen, m.configRevision
	if backend == nil || m.closed || m.conn.Status != domain.ConnectionConnected {
		m.mu.Unlock()
		return nil, ErrUnavailable
	}
	select {
	case m.operations <- struct{}{}:
	default:
		m.mu.Unlock()
		return nil, ErrBusy
	}
	m.workerWG.Add(1)
	workerCtx := m.workerCtx
	if workerCtx == nil {
		workerCtx = m.ctx
	}
	m.mu.Unlock()
	defer m.workerWG.Done()
	defer func() { <-m.operations }()
	readCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(workerCtx, cancel)
	defer func() { stop(); cancel() }()
	discovery, err := m.readDiscoveryAt(readCtx, gen, backend, revision)
	if err != nil {
		// A canceled SDK read closes its transport. Invalidate cached evidence
		// synchronously; recovery belongs to the connection lifecycle, not the
		// canceled HTTP request.
		m.handleTransportLoss(workerCtx, gen, backend)
		return nil, ErrUnavailable
	}
	m.mu.Lock()
	if m.gen != gen || m.backend != backend || m.closed || m.configRevision != revision || readCtx.Err() != nil {
		m.mu.Unlock()
		return nil, ErrUnavailable
	}
	m.discovery = discovery
	m.conn.SceneCollection = discovery.SceneCollection
	m.mu.Unlock()
	m.notify()
	return copyDiscovery(discovery), nil
}

func (m *Manager) Disconnect(ctx context.Context) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.mu.Lock()
	if m.workerCancel != nil {
		m.workerCancel()
	}
	m.gen++
	m.needsRevalidation = true
	backend := m.backend
	m.backend = nil
	m.discovery = nil
	m.observation = nil
	m.conn = domain.OBSConnection{Status: domain.ConnectionNotConnected, DetailCode: "disconnected", Detail: "OBS is disconnected.", Address: m.endpoint, Generation: m.gen, ConfigRevision: m.configRevision}
	m.mu.Unlock()
	if backend != nil {
		_ = backend.Close()
	}
	m.workerWG.Wait()
	m.notify()
	return nil
}

func (m *Manager) Close() {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.gen++
	if m.workerCancel != nil {
		m.workerCancel()
	}
	backend := m.backend
	m.backend = nil
	m.mu.Unlock()
	m.cancel()
	if backend != nil {
		_ = backend.Close()
	}
	m.workerWG.Wait()
}

func (m *Manager) current(gen uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gen == gen && !m.closed
}

func (m *Manager) fail(gen uint64, code, detail string) {
	m.setState(gen, domain.ConnectionNeedsAttention, code, detail)
}

func (m *Manager) setState(gen uint64, status, code, detail string) {
	m.mu.Lock()
	if m.gen != gen || m.closed {
		m.mu.Unlock()
		return
	}
	m.conn.Status = status
	m.conn.DetailCode = code
	m.conn.Detail = detail
	m.mu.Unlock()
	m.notify()
}

func (m *Manager) notify() {
	if m.onUpdate != nil {
		m.onUpdate()
	}
}

func (m *Manager) sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func passwordKey(endpoint string) string {
	return "obs-password:" + strings.TrimSpace(endpoint)
}
