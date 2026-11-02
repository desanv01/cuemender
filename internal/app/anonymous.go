package app

import (
	"context"
	"github.com/desanv01/cuemender/internal/domain"
)

func (a *App) chatSnapshot() domain.TwitchConnection {
	if a.anonymous != nil {
		return a.anonymous.Snapshot()
	}
	if a.twitchMgr != nil {
		return a.twitchMgr.Snapshot()
	}
	return domain.TwitchConnection{Status: domain.ConnectionNotConnected}
}
func (a *App) ReceiveOnlyConnect(ctx context.Context, channel string, confirmed bool) error {
	if a.anonymous == nil {
		return domain.ErrModeConflict
	}
	if !confirmed || channel != a.receiveOnlyChannel {
		return domain.ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	a.integrationMu.Lock()
	defer a.integrationMu.Unlock()
	if err := a.anonymous.Connect(); err != nil {
		return domain.ErrBusy
	}
	return nil
}
