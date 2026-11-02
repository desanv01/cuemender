package twitch

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

const (
	replyHelp = "CueMender commands: !issueaudio, !issuevideo, !fixed CODE, !stillbroken CODE, !streamhelp. Monitoring is not active yet."
	replyAck  = "CueMender received your report. Monitoring is not active yet, so no incident was opened."
)

var (
	ErrCredentialStorageUnavailable = errors.New("credential storage is unavailable")
	ErrAuthorizationRevoked         = errors.New("twitch authorization is revoked")
)

type Options struct {
	Context               context.Context
	OAuth                 *OAuth
	Store                 *CredentialStore
	Dialer                Dialer
	Endpoints             Endpoints
	ClientID              string
	Now                   func() time.Time
	OnUpdate              func()
	OnCommand             func(CommandEvent)
	SuppressStaticReplies bool
	StorageMode           string
	RefreshPeriod         time.Duration
	// MaintenanceWait is a deterministic timer seam; nil uses a cancellable timer.
	MaintenanceWait func(context.Context, time.Duration) bool
	RetryWait       func(context.Context, time.Duration) bool
}

type Manager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	oauth           *OAuth
	store           *CredentialStore
	dialer          Dialer
	endpoints       Endpoints
	now             func() time.Time
	onUpdate        func()
	onCommand       func(CommandEvent)
	suppressStatic  bool
	storageMode     string
	refreshPeriod   time.Duration
	maintenanceWait func(context.Context, time.Duration) bool
	retryWait       func(context.Context, time.Duration) bool

	lifecycleMu   sync.Mutex
	maintenanceMu sync.Mutex
	credentialMu  sync.Mutex
	mu            sync.Mutex
	refreshMu     sync.Mutex
	refreshOnce   sync.Once
	refreshWG     sync.WaitGroup
	authCtx       context.Context
	authCancel    context.CancelFunc
	gen           uint64
	connGen       uint64
	connectedAt   time.Time
	conn          domain.TwitchConnection
	creds         *Credentials
	clientID      string
	deviceCode    string
	reply         *ReplyQueue
	workerCancel  context.CancelFunc
	workerWG      sync.WaitGroup
	operationWG   sync.WaitGroup
	commandWG     sync.WaitGroup
	closed        bool
	dedup         *Dedup
	cooldown      *Cooldown
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
	if opts.RefreshPeriod <= 0 {
		opts.RefreshPeriod = time.Hour
	}
	if opts.Endpoints.EventSub == "" {
		opts.Endpoints = DefaultEndpoints()
	}
	m := &Manager{
		ctx:             managerCtx,
		cancel:          cancel,
		oauth:           opts.OAuth,
		store:           opts.Store,
		dialer:          opts.Dialer,
		endpoints:       opts.Endpoints,
		now:             opts.Now,
		onUpdate:        opts.OnUpdate,
		onCommand:       opts.OnCommand,
		suppressStatic:  opts.SuppressStaticReplies,
		storageMode:     opts.StorageMode,
		refreshPeriod:   opts.RefreshPeriod,
		maintenanceWait: opts.MaintenanceWait,
		retryWait:       opts.RetryWait,
		clientID:        opts.ClientID,
		dedup:           NewDedup(),
		cooldown:        NewCooldown(10 * time.Second),
		conn: domain.TwitchConnection{
			Status:             domain.ConnectionNotConnected,
			ClientIDConfigured: opts.ClientID != "",
		},
	}
	if m.oauth != nil && m.oauth.Endpoints.EventSub == "" {
		m.oauth.Endpoints = m.endpoints
	}
	m.authCtx, m.authCancel = context.WithCancel(managerCtx)
	return m
}

func (m *Manager) Snapshot() domain.TwitchConnection {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn := m.conn
	if conn.Device != nil {
		device := *conn.Device
		conn.Device = &device
	}
	return conn
}

func (m *Manager) Connect(clientID string) error {
	if !ValidClientID(clientID) {
		return ErrInvalidClientID
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	if m.storageMode == domain.SecretsModeUnavailable {
		return ErrCredentialStorageUnavailable
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrBusy
	}
	switch m.conn.Status {
	case domain.ConnectionConnecting, domain.ConnectionAuthorizing, domain.ConnectionConnected, domain.ConnectionReconnecting:
		m.mu.Unlock()
		return ErrBusy
	}
	m.mu.Unlock()
	m.stopAuthorization()
	m.mu.Lock()
	m.authCtx, m.authCancel = context.WithCancel(m.ctx)
	ctx, cancel := context.WithCancel(m.authCtx)
	m.workerCancel = cancel
	m.gen++
	m.connGen++
	gen := m.gen
	m.clientID = clientID
	m.creds = nil
	m.deviceCode = ""
	m.conn = domain.TwitchConnection{
		Status:             domain.ConnectionAuthorizing,
		DetailCode:         "authorizing",
		Detail:             "Complete the device authorization on Twitch.",
		ClientIDConfigured: true,
		Receipt:            m.conn.Receipt,
	}
	m.workerWG.Add(1)
	m.mu.Unlock()
	m.notify()
	go func() {
		defer m.workerWG.Done()
		m.deviceFlow(ctx, gen, clientID)
	}()
	return nil
}

func (m *Manager) deviceFlow(ctx context.Context, gen uint64, clientID string) {
	if m.store != nil {
		if old, err := m.store.Load(ctx); err == nil && old.ClientID != clientID {
			_ = m.store.Delete(ctx)
		}
	}
	start, err := m.oauth.StartDevice(ctx, clientID)
	if err != nil {
		m.fail(gen, "authorization_start_failed", "Twitch authorization could not be started.")
		return
	}
	m.mu.Lock()
	if !m.currentLocked(gen) {
		m.mu.Unlock()
		return
	}
	m.deviceCode = start.DeviceCode
	m.conn.Status = domain.ConnectionAuthorizing
	m.conn.DetailCode = "device_pending"
	m.conn.Detail = "Enter the code on Twitch, then return to CueMender."
	m.conn.Device = &domain.DeviceAuthorization{
		VerificationURI: start.VerificationURI,
		UserCode:        start.UserCode,
		ExpiresAt:       m.now().Add(start.ExpiresIn),
	}
	m.mu.Unlock()
	m.notify()

	creds, err := m.oauth.PollDevice(ctx, clientID, start)
	if err != nil {
		if errors.Is(err, context.Canceled) || !m.current(gen) {
			return
		}
		var providerErr *ProviderError
		if errors.As(err, &providerErr) {
			switch providerErr.Code {
			case "access_denied":
				m.fail(gen, "device_denied", "Twitch authorization was denied.")
			case "expired_token":
				m.fail(gen, "device_expired", "The Twitch authorization code expired. Start again.")
			default:
				m.fail(gen, "authorization_failed", "Twitch authorization failed.")
			}
			return
		}
		m.fail(gen, "authorization_failed", "Twitch authorization failed.")
		return
	}
	if !m.current(gen) {
		return
	}
	info, err := m.validateStartup(ctx, gen, creds.AccessToken)
	if err != nil {
		m.fail(gen, "token_invalid", "Twitch rejected the new authorization.")
		return
	}
	if info.ClientID != clientID || !hasAllScopes(info.Scopes, []string{ScopeReadChat, ScopeWriteChat}) || info.ExpiresIn <= 0 {
		m.fail(gen, "token_binding_mismatch", "Twitch returned an authorization for the wrong client or scopes.")
		return
	}
	creds.UserID = info.UserID
	creds.Login = info.Login
	creds.Scopes = info.Scopes
	creds.ExpiresAt = m.now().Add(info.ExpiresIn)
	persistenceDegraded := false
	if err := m.saveCredentials(ctx, gen, creds); err != nil {
		// Keep the in-memory session but report that it will not survive
		// restart; no plaintext fallback is attempted.
		persistenceDegraded = true
		m.mu.Lock()
		if m.currentLocked(gen) {
			m.conn.CredentialPersistenceDegraded = true
			m.conn.DetailCode = "credential_persistence_degraded"
			m.conn.Detail = "Connected for this run, but the credential could not be saved."
		}
		m.mu.Unlock()
		m.notify()
	}
	m.mu.Lock()
	if !m.currentLocked(gen) {
		m.mu.Unlock()
		return
	}
	m.creds = &creds
	m.conn.Status = domain.ConnectionConnecting
	if !persistenceDegraded {
		m.conn.DetailCode = "event_connecting"
		m.conn.Detail = "Twitch authorization is valid; connecting chat delivery."
	}
	m.conn.Login = info.Login
	m.conn.Device = nil
	m.conn.ClientIDConfigured = true
	m.mu.Unlock()
	m.startReplyQueue(gen)
	m.notify()
	m.startEventSub(ctx, gen)
}

func (m *Manager) Disconnect(ctx context.Context) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.stopAuthorization()
	m.mu.Lock()
	m.conn = domain.TwitchConnection{
		Status:             domain.ConnectionNotConnected,
		DetailCode:         "disconnected",
		Detail:             "Twitch is disconnected.",
		ClientIDConfigured: m.clientID != "",
	}
	m.mu.Unlock()
	m.credentialMu.Lock()
	if m.store != nil {
		err := m.store.Delete(ctx)
		m.credentialMu.Unlock()
		if err != nil {
			m.mu.Lock()
			m.conn.Status = domain.ConnectionNeedsAttention
			m.conn.DetailCode = "credential_delete_failed"
			m.conn.Detail = "Twitch is disconnected, but stored authorization could not be removed."
			m.mu.Unlock()
			m.notify()
			return ErrCredentialDeleteFailed
		}
	} else {
		m.credentialMu.Unlock()
	}
	m.notify()
	return nil
}

// Called with lifecycleMu held. Invalidating under mu closes operation admission
// before waiting, so no Add can race a zero-count Wait or revive this lifecycle.
func (m *Manager) stopAuthorization() {
	m.mu.Lock()
	if m.workerCancel != nil {
		m.workerCancel()
	}
	if m.authCancel != nil {
		m.authCancel()
	}
	m.gen++
	m.creds = nil
	m.deviceCode = ""
	reply := m.reply
	m.reply = nil
	m.workerCancel = nil
	m.mu.Unlock()
	if reply != nil {
		reply.Close()
	}
	m.maintenanceMu.Lock()
	m.maintenanceMu.Unlock()
	m.operationWG.Wait()
	m.commandWG.Wait()
	m.workerWG.Wait()
}

// StartSaved resumes a validated saved authorization. It never begins a new
// device flow spontaneously.
func (m *Manager) StartSaved() {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.mu.Lock()
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return
	}
	if m.storageMode == domain.SecretsModeUnavailable {
		m.mu.Lock()
		m.conn.Status = domain.ConnectionNeedsAttention
		m.conn.DetailCode = "credential_storage_unavailable"
		m.conn.Detail = "OS credential storage is unavailable. Restart with --session-only-secrets for memory-only mode."
		m.mu.Unlock()
		m.notify()
		return
	}
	m.mu.Lock()
	if m.closed || m.workerCancel != nil {
		m.mu.Unlock()
		return
	}
	clientID := m.clientID
	m.mu.Unlock()
	if clientID == "" || !ValidClientID(clientID) || m.store == nil {
		m.mu.Lock()
		m.conn.Status = domain.ConnectionNotConnected
		m.conn.DetailCode = "not_configured"
		m.conn.Detail = "Add a Twitch Public client ID and authorize this channel."
		m.mu.Unlock()
		m.notify()
		return
	}
	m.mu.Lock()
	if m.authCancel != nil {
		m.authCancel()
	}
	m.authCtx, m.authCancel = context.WithCancel(m.ctx)
	ctx, cancel := context.WithCancel(m.authCtx)
	m.workerCancel = cancel
	m.gen++
	gen := m.gen
	m.conn.Status = domain.ConnectionConnecting
	m.conn.DetailCode = "validating_saved_credentials"
	m.conn.Detail = "Checking the saved Twitch authorization."
	m.workerWG.Add(1)
	m.mu.Unlock()
	m.notify()
	go func() {
		defer m.workerWG.Done()
		m.resumeFlow(ctx, gen, clientID)
	}()
}

func (m *Manager) resumeFlow(ctx context.Context, gen uint64, clientID string) {
	creds, err := m.store.Load(ctx)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoCredentials):
			m.setState(gen, domain.ConnectionNotConnected, "not_authorized", "Authorize Twitch to connect your own channel.")
		case errors.Is(err, ErrCredentialsUnavailable):
			m.fail(gen, "credential_storage_unavailable", "Stored Twitch authorization is unavailable.")
		default:
			m.fail(gen, "authorization_required", "Stored Twitch authorization is invalid. Authorize again.")
		}
		return
	}
	originalUserID := creds.UserID
	if creds.ClientID != clientID {
		_ = m.store.Delete(ctx)
		m.fail(gen, "authorization_required", "The saved Twitch authorization belongs to a different client.")
		return
	}
	info, err := m.validateStartup(ctx, gen, creds.AccessToken)
	if err != nil && !isTerminalAuthError(err) {
		return
	}
	if err == nil && (info.ClientID != clientID || (originalUserID != "" && info.UserID != originalUserID) || !hasAllScopes(info.Scopes, []string{ScopeReadChat, ScopeWriteChat})) {
		m.fail(gen, "authorization_required", "The saved Twitch authorization belongs to a different client, channel or scopes.")
		return
	}
	if err != nil || info.ExpiresIn <= 0 {
		creds, err = m.oauth.Refresh(ctx, clientID, creds.RefreshToken)
		if err != nil {
			if isTerminalAuthError(err) {
				m.fail(gen, "authorization_required", "The saved Twitch authorization expired or was revoked.")
			} else {
				m.setState(gen, domain.ConnectionNeedsAttention, "authorization_unavailable", "Twitch authorization could not be checked. Try connecting again.")
			}
			return
		}
	}
	if !m.current(gen) {
		return
	}
	info, err = m.validateStartup(ctx, gen, creds.AccessToken)
	if err != nil {
		if isTerminalAuthError(err) {
			m.fail(gen, "authorization_required", "The saved Twitch authorization is not valid.")
		}
		return
	}
	if info.ClientID != clientID || !hasAllScopes(info.Scopes, []string{ScopeReadChat, ScopeWriteChat}) || info.ExpiresIn <= 0 {
		m.fail(gen, "authorization_required", "The saved Twitch authorization is not valid.")
		return
	}
	if creds.UserID != "" && info.UserID != creds.UserID {
		m.fail(gen, "authorization_required", "The saved Twitch authorization belongs to a different channel identity.")
		return
	}
	if originalUserID != "" && info.UserID != originalUserID {
		m.fail(gen, "authorization_required", "The refreshed authorization belongs to a different channel identity.")
		return
	}
	creds.UserID = info.UserID
	creds.Login = info.Login
	creds.Scopes = info.Scopes
	creds.ExpiresAt = m.now().Add(info.ExpiresIn)
	persistenceDegraded := false
	if err := m.saveCredentials(ctx, gen, creds); err != nil {
		persistenceDegraded = true
		m.mu.Lock()
		if m.currentLocked(gen) {
			m.conn.CredentialPersistenceDegraded = true
			m.conn.DetailCode = "credential_persistence_degraded"
			m.conn.Detail = "Connected for this run, but the credential could not be saved."
		}
		m.mu.Unlock()
	}
	m.mu.Lock()
	if !m.currentLocked(gen) {
		m.mu.Unlock()
		return
	}
	m.creds = &creds
	m.conn.Status = domain.ConnectionConnecting
	if !persistenceDegraded {
		m.conn.DetailCode = "event_connecting"
		m.conn.Detail = "Twitch authorization is valid; connecting chat delivery."
	}
	m.conn.Login = info.Login
	m.conn.ClientIDConfigured = true
	m.mu.Unlock()
	m.startReplyQueue(gen)
	m.notify()
	m.startEventSub(ctx, gen)
}

// A timeout, rate limit or provider outage is not evidence of revocation.
func (m *Manager) validateStartup(ctx context.Context, gen uint64, token string) (TokenInfo, error) {
	return m.validateWithRetry(ctx, gen, token, true)
}

func (m *Manager) validateWithRetry(ctx context.Context, gen uint64, token string, startup bool) (TokenInfo, error) {
	backoff := time.Second
	for {
		info, err := m.oauth.Validate(ctx, token)
		if err == nil || isTerminalAuthError(err) || ctx.Err() != nil || !m.current(gen) {
			return info, err
		}
		if startup {
			m.setState(gen, domain.ConnectionReconnecting, "validation_retry", "Twitch authorization validation is temporarily unavailable; retrying.")
		}
		if !m.sleep(ctx, backoff) {
			return TokenInfo{}, ctx.Err()
		}
		backoff = nextBackoff(backoff)
	}
}

func (m *Manager) startEventSub(ctx context.Context, gen uint64) {
	m.mu.Lock()
	if !m.currentLocked(gen) || ctx.Err() != nil {
		m.mu.Unlock()
		return
	}
	m.workerWG.Add(1)
	m.mu.Unlock()
	go func() {
		defer m.workerWG.Done()
		backoff := time.Second
		for {
			if !m.current(gen) || ctx.Err() != nil {
				return
			}
			creds := m.currentCredentials()
			if creds == nil {
				m.fail(gen, "authorization_required", "Twitch authorization is required.")
				return
			}
			sock, err := m.dialer.Dial(ctx, m.endpoints.EventSub, nil)
			if err != nil {
				m.setState(gen, domain.ConnectionReconnecting, "event_connection_failed", "Twitch chat connection is retrying.")
				if !m.sleep(ctx, backoff) {
					return
				}
				backoff = nextBackoff(backoff)
				continue
			}
			m.mu.Lock()
			before := m.connGen
			m.mu.Unlock()
			err = m.runSocket(ctx, gen, sock, *creds, true)
			// A connection surviving a full keepalive interval is stable recovery.
			now := m.now()
			m.mu.Lock()
			stable := m.gen == gen && m.connGen > before && now.Sub(m.connectedAt) >= 30*time.Second
			m.mu.Unlock()
			if stable {
				backoff = time.Second
			}
			_ = sock.Close()
			if ctx.Err() != nil || !m.current(gen) {
				return
			}
			if err != nil {
				if errors.Is(err, ErrAuthorizationRevoked) {
					m.failAuth(gen, "authorization_required", "Twitch authorization was revoked or rejected.")
					return
				}
				if isTerminalAuthError(err) && m.failAuthForCredentials(gen, *creds, "authorization_required", "Twitch authorization was revoked or rejected.") {
					return
				}
				m.setState(gen, domain.ConnectionReconnecting, "event_connection_lost", "Twitch chat connection was lost; reconnecting.")
				if !m.sleep(ctx, backoff) {
					return
				}
				backoff = nextBackoff(backoff)
				continue
			}
			return
		}
	}()
}

func (m *Manager) handover(ctx context.Context, gen uint64, oldSock Socket, keepalive time.Duration, reconnectURL string) (Socket, SessionInfo, time.Duration, error) {
	handoverCtx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	type replacement struct {
		sock    Socket
		session SessionInfo
		err     error
	}
	replacementCh := make(chan replacement, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		newSock, err := m.dialer.Dial(handoverCtx, reconnectURL, nil)
		if err != nil {
			replacementCh <- replacement{err: err}
			return
		}
		welcome, err := readEnvelope(handoverCtx, newSock, 10*time.Second)
		if err != nil {
			_ = newSock.Close()
			replacementCh <- replacement{err: err}
			return
		}
		session, err := ParseWelcome(welcome)
		if err != nil {
			_ = newSock.Close()
			replacementCh <- replacement{err: err}
			return
		}
		replacementCh <- replacement{sock: newSock, session: session}
	}()

	type oldFrame struct {
		env EventEnvelope
		err error
	}
	oldFrames := make(chan oldFrame, 8)
	workers.Add(1)
	go func() {
		defer workers.Done()
		defer close(oldFrames)
		for {
			env, err := readEnvelope(handoverCtx, oldSock, keepalive+5*time.Second)
			select {
			case oldFrames <- oldFrame{env: env, err: err}:
			case <-handoverCtx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	// Both readers are owned and joined on every exit, including old-socket
	// failure while the replacement welcome is blocked. A late successful dial
	// is still ours to close unless ownership was transferred to runSocket.
	defer func() {
		cancel()
		_ = oldSock.Close()
		workers.Wait()
		select {
		case orphan := <-replacementCh:
			if orphan.sock != nil {
				_ = orphan.sock.Close()
			}
		default:
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil, SessionInfo{}, 0, ctx.Err()
		case got := <-replacementCh:
			if got.err != nil {
				return nil, SessionInfo{}, 0, got.err
			}
			_ = oldSock.Close()
			// Consume frames already read before the handover completed.
			for frame := range oldFrames {
				if frame.err == nil && frame.env.Metadata.MessageType == "notification" {
					if notification, err := ParseChatNotification(frame.env); err == nil {
						m.handleChat(gen, notification.Event, frame.env.Metadata.MessageID)
					}
				}
				if frame.env.Metadata.MessageType == "revocation" {
					_ = got.sock.Close()
					return nil, SessionInfo{}, 0, ErrAuthorizationRevoked
				}
			}
			workers.Wait()
			if ctx.Err() != nil || !m.current(gen) {
				_ = got.sock.Close()
				return nil, SessionInfo{}, 0, context.Canceled
			}
			return got.sock, got.session, keepaliveDuration(got.session), nil
		case frame, ok := <-oldFrames:
			if !ok {
				return nil, SessionInfo{}, 0, ErrProtocol
			}
			if frame.err != nil {
				return nil, SessionInfo{}, 0, frame.err
			}
			switch frame.env.Metadata.MessageType {
			case "session_keepalive":
				continue
			case "notification":
				notification, err := ParseChatNotification(frame.env)
				if err != nil {
					continue
				}
				m.handleChat(gen, notification.Event, frame.env.Metadata.MessageID)
			case "revocation":
				return nil, SessionInfo{}, 0, ErrAuthorizationRevoked
			}
		}
	}
}

func (m *Manager) runSocket(ctx context.Context, gen uint64, sock Socket, creds Credentials, subscribe bool) error {
	defer func() { _ = sock.Close() }()
	env, err := readEnvelope(ctx, sock, 10*time.Second)
	if err != nil {
		return err
	}
	session, err := ParseWelcome(env)
	if err != nil {
		return err
	}
	if subscribe {
		if err := m.oauth.SubscribeChat(ctx, creds.AccessToken, creds.ClientID, session.ID, creds.UserID); err != nil {
			return err
		}
	}
	if !m.markConnected(gen, "Twitch chat is connected for the broadcaster's own channel.") {
		return context.Canceled
	}
	keepalive := keepaliveDuration(session)
	for {
		if !m.current(gen) || ctx.Err() != nil {
			return ctx.Err()
		}
		env, err = readEnvelope(ctx, sock, keepalive+5*time.Second)
		if err != nil {
			return err
		}
		switch env.Metadata.MessageType {
		case "session_keepalive":
			continue
		case "notification":
			notification, err := ParseChatNotification(env)
			if err != nil {
				continue
			}
			m.handleChat(gen, notification.Event, env.Metadata.MessageID)
		case "session_reconnect":
			reconnect, err := ParseReconnect(env)
			if err != nil {
				return err
			}
			if err := ValidateReconnectURL(reconnect.ReconnectURL); err != nil {
				return err
			}
			newSock, newSession, newKeepalive, err := m.handover(ctx, gen, sock, keepalive, reconnect.ReconnectURL)
			if err != nil {
				return err
			}
			sock = newSock
			session = newSession
			keepalive = newKeepalive
			if !m.markConnected(gen, "Twitch chat reconnected.") {
				return context.Canceled
			}
		case "revocation":
			return ErrAuthorizationRevoked
		default:
			return ErrProtocol
		}
	}
}

func (m *Manager) markConnected(gen uint64, detail string) bool {
	now := m.now()
	m.mu.Lock()
	if !m.currentLocked(gen) {
		m.mu.Unlock()
		return false
	}
	m.connGen++
	m.connectedAt = now
	m.conn.ConnectionGeneration = m.connGen
	m.conn.Status = domain.ConnectionConnected
	if !m.conn.CredentialPersistenceDegraded {
		m.conn.DetailCode = "connected"
		m.conn.Detail = detail
	}
	m.mu.Unlock()
	m.notify()
	return true
}

func (m *Manager) handleChat(gen uint64, event ChatEvent, deliveryID string) {
	now := m.now()
	cmd, ok := ParseCommand(event.Message.Text)
	if !ok {
		return
	}
	m.mu.Lock()
	if !m.currentLocked(gen) || m.creds == nil || event.BroadcasterUserID != m.creds.UserID || event.ChatterUserID == m.creds.UserID {
		m.mu.Unlock()
		return
	}
	if m.dedup.Seen(deliveryID, event.MessageID, now) {
		m.mu.Unlock()
		return
	}
	if cmd.IsIssue() && !m.cooldown.Allow(event.ChatterUserID, now) {
		m.mu.Unlock()
		return
	}
	reply := m.reply
	m.commandWG.Add(1)
	m.conn.Receipt.Accepted++
	m.conn.Receipt.LastCommand = string(cmd.Kind)
	m.conn.Receipt.LastCategory = cmd.Category
	ts := now
	m.conn.Receipt.LastAt = &ts
	connectionGeneration := m.conn.ConnectionGeneration
	m.mu.Unlock()
	defer m.commandWG.Done()
	m.notify()
	if m.onCommand != nil {
		m.onCommand(CommandEvent{
			DeliveryID:           deliveryID,
			MessageID:            event.MessageID,
			BroadcasterID:        event.BroadcasterUserID,
			ActorID:              event.ChatterUserID,
			ReceivedAt:           now,
			Command:              cmd,
			ConnectionGeneration: connectionGeneration,
		})
	}
	if reply != nil && !m.suppressStatic {
		switch cmd.Kind {
		case CommandStreamHelp:
			reply.Enqueue(replyHelp)
		case CommandIssueAudio, CommandIssueVideo:
			reply.Enqueue(replyAck)
		}
	}
}

// EnqueueReply lets the P03 coordinator use the existing bounded queue for
// durable, generation-bound acknowledgments.
func (m *Manager) EnqueueReply(text string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reply == nil || m.closed {
		return false
	}
	return m.reply.Enqueue(text)
}

func (m *Manager) EnqueueReplyFor(generation uint64, text string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reply == nil || m.closed || m.conn.ConnectionGeneration != generation {
		return false
	}
	return m.reply.Enqueue(text)
}

func (m *Manager) startReplyQueue(gen uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reply != nil {
		return
	}
	if !m.currentLocked(gen) {
		return
	}
	m.reply = NewReplyQueue(m.authCtx, func(ctx context.Context, text string) error {
		m.mu.Lock()
		if !m.currentLocked(gen) || m.creds == nil {
			m.mu.Unlock()
			return context.Canceled
		}
		creds := cloneCredentials(*m.creds)
		m.mu.Unlock()
		err := m.oauth.SendChat(ctx, *creds, creds.UserID, text)
		if isTerminalAuthError(err) {
			m.failAuthForCredentials(gen, *creds, "authorization_required", "Twitch rejected chat authorization.")
		}
		return err
	}, m.now, 3*time.Second, m.oauth.chatDelay)
}

// RefreshNow is used by the hourly loop and tests. It serializes refresh and
// discards rotated credentials if the lifecycle changed during the call.
func (m *Manager) RefreshNow(ctx context.Context) error {
	ctx, gen, finish, err := m.beginOperation(ctx)
	if err != nil {
		return err
	}
	defer finish()
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	m.mu.Lock()
	if !m.currentLocked(gen) || m.creds == nil || ctx.Err() != nil {
		m.mu.Unlock()
		return context.Canceled
	}
	creds := cloneCredentials(*m.creds)
	m.mu.Unlock()
	refreshed, err := m.oauth.Refresh(ctx, creds.ClientID, creds.RefreshToken)
	if err != nil {
		if isTerminalAuthError(err) {
			m.failAuth(gen, "authorization_required", "Twitch authorization expired or was revoked.")
		}
		return err
	}
	refreshed.UserID = creds.UserID
	refreshed.Login = creds.Login
	if refreshed.ClientID == "" {
		refreshed.ClientID = creds.ClientID
	}
	// Validate the rotated token before trusting it. Missing scopes are never
	// restored from the old credential.
	info, err := m.validateWithRetry(ctx, gen, refreshed.AccessToken, false)
	if err != nil {
		if isTerminalAuthError(err) {
			m.failAuth(gen, "authorization_required", "The rotated Twitch authorization is no longer valid.")
		}
		return err
	}
	if info.ClientID != creds.ClientID || info.UserID != creds.UserID ||
		!hasAllScopes(info.Scopes, []string{ScopeReadChat, ScopeWriteChat}) || info.ExpiresIn <= 0 {
		m.failAuth(gen, "authorization_required", "The rotated Twitch authorization is no longer valid.")
		return ErrAuthorizationRevoked
	}
	refreshed.UserID = info.UserID
	refreshed.Login = info.Login
	refreshed.Scopes = info.Scopes
	refreshed.ExpiresAt = m.now().Add(info.ExpiresIn)
	persistErr := m.saveCredentials(ctx, gen, refreshed)
	if persistErr != nil {
		m.mu.Lock()
		if m.currentLocked(gen) {
			m.conn.CredentialPersistenceDegraded = true
			m.conn.DetailCode = "credential_persistence_degraded"
			m.conn.Detail = "Connected for this run, but the rotated credential could not be saved."
		}
		m.mu.Unlock()
		m.notify()
	}
	m.mu.Lock()
	if !m.currentLocked(gen) || ctx.Err() != nil {
		m.mu.Unlock()
		return context.Canceled
	}
	// A validated rotated pair is the only usable pair after rotation; keep it
	// for the established session even when durable persistence failed.
	m.creds = &refreshed
	m.mu.Unlock()
	m.notify()
	return persistErr
}

// Register under the same lock that invalidates credentials. Caller deadlines
// and authorization cancellation both cancel the provider operation.
func (m *Manager) beginOperation(caller context.Context) (context.Context, uint64, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.creds == nil || m.authCtx.Err() != nil {
		return nil, 0, nil, ErrNoCredentials
	}
	if err := caller.Err(); err != nil {
		return nil, 0, nil, err
	}
	ctx, cancel := context.WithCancel(m.authCtx)
	stop := context.AfterFunc(caller, cancel)
	m.operationWG.Add(1)
	return ctx, m.gen, func() { stop(); cancel(); m.operationWG.Done() }, nil
}

func (m *Manager) saveCredentials(ctx context.Context, gen uint64, creds Credentials) error {
	if m.store == nil {
		return ErrCredentialsUnavailable
	}
	m.credentialMu.Lock()
	defer m.credentialMu.Unlock()
	if !m.current(gen) {
		return context.Canceled
	}
	if err := m.store.Save(ctx, creds); err != nil {
		return err
	}
	if !m.current(gen) {
		// The lifecycle was disconnected while persistence was in flight.
		// Remove the late write so a successful Disconnect cannot leave a
		// resurrected credential behind.
		_ = m.store.Delete(context.Background())
		return context.Canceled
	}
	return nil
}

func (m *Manager) currentCredentials() *Credentials {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.creds == nil {
		return nil
	}
	return cloneCredentials(*m.creds)
}

func cloneCredentials(creds Credentials) *Credentials {
	creds.Scopes = append([]string(nil), creds.Scopes...)
	return &creds
}

func (m *Manager) current(gen uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentLocked(gen)
}

func (m *Manager) currentLocked(gen uint64) bool {
	return m.gen == gen && !m.closed && m.authCtx.Err() == nil
}

func (m *Manager) fail(gen uint64, code, detail string) {
	m.failAuth(gen, code, detail)
}

// failAuth is a terminal authorization decision: it stops intake, replies,
// reconnect attempts and device work instead of only changing status copy.
func (m *Manager) failAuth(gen uint64, code, detail string) {
	m.mu.Lock()
	if !m.currentLocked(gen) {
		m.mu.Unlock()
		return
	}
	reply, cancel := m.failAuthLocked(code, detail)
	m.mu.Unlock()
	m.finishAuthFailure(reply, cancel)
}

// External request failures wait for a rotation already in flight, then compare
// the exact credential pair atomically with the terminal state transition.
// RefreshNow itself uses failAuth because it already owns refreshMu.
func (m *Manager) failAuthForCredentials(gen uint64, expected Credentials, code, detail string) bool {
	m.refreshMu.Lock()
	m.mu.Lock()
	if !m.currentLocked(gen) || m.creds == nil ||
		m.creds.AccessToken != expected.AccessToken || m.creds.RefreshToken != expected.RefreshToken ||
		m.creds.ClientID != expected.ClientID || m.creds.UserID != expected.UserID {
		m.mu.Unlock()
		m.refreshMu.Unlock()
		return false
	}
	reply, cancel := m.failAuthLocked(code, detail)
	m.mu.Unlock()
	m.refreshMu.Unlock()
	m.finishAuthFailure(reply, cancel)
	return true
}

func (m *Manager) failAuthLocked(code, detail string) (*ReplyQueue, context.CancelFunc) {
	m.conn.Status = domain.ConnectionNeedsAttention
	m.conn.DetailCode = code
	m.conn.Detail = detail
	m.conn.Device = nil
	reply := m.reply
	m.creds = nil
	m.authCancel()
	cancel := m.workerCancel
	return reply, cancel
}

func (m *Manager) finishAuthFailure(reply *ReplyQueue, cancel context.CancelFunc) {
	if cancel != nil {
		cancel()
	}
	if reply != nil {
		reply.stop()
	}
	m.notify()
}

func (m *Manager) setState(gen uint64, status, code, detail string) {
	m.mu.Lock()
	if !m.currentLocked(gen) {
		m.mu.Unlock()
		return
	}
	m.conn.Status = status
	if !(status == domain.ConnectionConnected && m.conn.CredentialPersistenceDegraded) {
		m.conn.DetailCode = code
		m.conn.Detail = detail
	}
	m.mu.Unlock()
	m.notify()
}

func (m *Manager) notify() {
	if m.onUpdate != nil {
		m.onUpdate()
	}
}

func (m *Manager) sleep(ctx context.Context, d time.Duration) bool {
	if m.retryWait != nil {
		return m.retryWait(ctx, d)
	}
	return waitTimer(ctx, d)
}

func waitTimer(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextBackoff(current time.Duration) time.Duration {
	current *= 2
	if current > 30*time.Second {
		return 30 * time.Second
	}
	return current
}

func isTerminalAuthError(err error) bool {
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Status == 401 || providerErr.Status == 403 || providerErr.Code == "invalid_token_expiry" || providerErr.Code == "invalid_grant"
	}
	return false
}

func readEnvelope(ctx context.Context, sock Socket, timeout time.Duration) (EventEnvelope, error) {
	readCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	data, err := sock.Read(readCtx)
	if err != nil {
		return EventEnvelope{}, err
	}
	return ParseEnvelope(data)
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
	m.mu.Unlock()
	m.cancel()
	m.stopAuthorization()
	m.refreshWG.Wait()
}

// StartRefreshLoop validates the saved token on the maintenance schedule and
// refreshes only when the validated token is near expiry. It exits on manager
// close and is joined by Close.
func (m *Manager) StartRefreshLoop() {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.mu.Lock()
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return
	}
	m.refreshOnce.Do(func() {
		period := m.refreshPeriod
		if period > 30*time.Second {
			period = 30 * time.Second
		}
		m.refreshWG.Add(1)
		go func() {
			defer m.refreshWG.Done()
			wait := m.maintenanceWait
			if wait == nil {
				wait = waitTimer
			}
			for {
				if !wait(m.ctx, period) {
					return
				}
				m.maintenance()
			}
		}()
	})
}

func (m *Manager) maintenance() {
	m.maintenanceMu.Lock()
	defer m.maintenanceMu.Unlock()
	m.mu.Lock()
	var credsCopy Credentials
	if m.creds != nil {
		credsCopy = *cloneCredentials(*m.creds)
	}
	gen := m.gen
	status := m.conn.Status
	ctx := m.authCtx
	m.mu.Unlock()
	if credsCopy.AccessToken == "" || status != domain.ConnectionConnected || m.oauth == nil || ctx.Err() != nil {
		return
	}
	if ctx == nil {
		ctx = m.ctx
	}
	info, err := m.oauth.Validate(ctx, credsCopy.AccessToken)
	if err != nil {
		if isTerminalAuthError(err) {
			m.failAuthForCredentials(gen, credsCopy, "authorization_required", "The saved Twitch authorization is no longer valid.")
		}
		return
	}
	if info.ClientID != credsCopy.ClientID || info.UserID != credsCopy.UserID ||
		!hasAllScopes(info.Scopes, []string{ScopeReadChat, ScopeWriteChat}) || info.ExpiresIn <= 0 {
		m.failAuthForCredentials(gen, credsCopy, "authorization_required", "The saved Twitch authorization is no longer valid.")
		return
	}
	m.mu.Lock()
	if !m.currentLocked(gen) || m.creds == nil || m.creds.AccessToken != credsCopy.AccessToken {
		m.mu.Unlock()
		return
	}
	now := m.now()
	m.creds.ExpiresAt = now.Add(info.ExpiresIn)
	m.creds.Login = info.Login
	m.creds.Scopes = info.Scopes
	needsRefresh := !m.creds.ExpiresAt.After(now.Add(5 * time.Minute))
	m.mu.Unlock()
	if needsRefresh {
		_ = m.RefreshNow(ctx)
	}
}
