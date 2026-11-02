package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/obs"
	"github.com/desanv01/cuemender/internal/twitch"
)

// onIntegrationUpdate serializes a provider state change through the accepted
// revision/publication boundary. Managers have already released their locks.
func (a *App) onIntegrationUpdate() {
	if a.coordinator != nil {
		a.coordinator.SignalIntegrationChange()
	}
	a.stateMu.Lock()
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
}

// TwitchConnect persists the non-secret public client ID before starting the
// asynchronous device flow.
func (a *App) TwitchConnect(ctx context.Context, clientID string) error {
	a.integrationMu.Lock()
	defer a.integrationMu.Unlock()
	if !twitch.ValidClientID(clientID) {
		return fmt.Errorf("%w: Twitch public client ID is invalid", domain.ErrInvalidInput)
	}
	if !a.secretsState.Available {
		return domain.ErrCredentialStorage
	}
	if busyStatus(a.twitchMgr.Snapshot().Status) {
		return domain.ErrBusy
	}
	if err := a.persistConfig(func(cfg *Config) { cfg.TwitchClientID = clientID }); err != nil {
		return err
	}
	if err := a.twitchMgr.Connect(clientID); err != nil {
		if errors.Is(err, twitch.ErrBusy) {
			return domain.ErrBusy
		}
		if errors.Is(err, twitch.ErrCredentialStorageUnavailable) {
			return domain.ErrCredentialStorage
		}
		return domain.ErrUnavailable
	}
	return nil
}

func (a *App) TwitchDisconnect(ctx context.Context) error {
	a.integrationMu.Lock()
	defer a.integrationMu.Unlock()
	if err := a.twitchMgr.Disconnect(ctx); err != nil {
		if errors.Is(err, twitch.ErrCredentialDeleteFailed) {
			return domain.ErrCredentialStorage
		}
		return domain.ErrUnavailable
	}
	if a.coordinator != nil {
		a.coordinator.Invalidate(ctx, "twitch_disconnected")
	}
	return nil
}

func (a *App) OBSConnect(ctx context.Context, address string, password *string) error {
	a.integrationMu.Lock()
	defer a.integrationMu.Unlock()
	if err := ValidateOBSAddress(address); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
	if busyStatus(a.obsMgr.Snapshot().Status) {
		return domain.ErrBusy
	}
	if err := a.persistConfig(func(cfg *Config) { cfg.OBSAddress = address }); err != nil {
		return err
	}
	if err := a.obsMgr.Connect(address, password); err != nil {
		switch {
		case errors.Is(err, obs.ErrBusy):
			return domain.ErrBusy
		case errors.Is(err, obs.ErrCredentialsUnavailable):
			return domain.ErrCredentialStorage
		default:
			return fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
		}
	}
	return nil
}

func busyStatus(status string) bool {
	switch status {
	case domain.ConnectionConnecting, domain.ConnectionAuthorizing, domain.ConnectionConnected, domain.ConnectionReconnecting:
		return true
	default:
		return false
	}
}

func (a *App) OBSDisconnect(ctx context.Context) error {
	a.integrationMu.Lock()
	defer a.integrationMu.Unlock()
	if err := a.obsMgr.Disconnect(ctx); err != nil {
		return domain.ErrUnavailable
	}
	if a.coordinator != nil {
		a.coordinator.Invalidate(ctx, "obs_disconnected")
	}
	return nil
}

func (a *App) OBSDiscovery(ctx context.Context) (domain.OBSDiscovery, error) {
	discovery, err := a.obsMgr.RefreshDiscovery(ctx)
	if err != nil || discovery == nil {
		return domain.OBSDiscovery{}, domain.ErrUnavailable
	}
	return *discovery, nil
}

// persistConfig serializes JSON config updates, preserves the configured
// listen address independently of a CLI override, and updates the effective
// non-secret startup values only after the atomic write succeeds.
func (a *App) persistConfig(update func(*Config)) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	next := a.fileConfig
	update(&next)
	if next.ListenAddress == "" {
		next.ListenAddress = a.fileConfig.ListenAddress
	}
	next.SchemaVersion = ConfigSchemaVersion
	if err := SaveConfig(a.dataDir, next); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
	a.stateMu.Lock()
	a.fileConfig = next
	a.cfg.TwitchClientID = next.TwitchClientID
	a.cfg.OBSAddress = next.OBSAddress
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
	return nil
}
