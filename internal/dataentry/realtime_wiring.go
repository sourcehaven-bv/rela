package dataentry

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/config"
)

// realtimeFile is the realtime configuration file this app watches. The
// name is spelled here rather than imported: dataentry needs only to
// subscribe to it and report on it, never to parse it.
const realtimeFile = "realtime.yaml"

// realtimeReloader re-reads realtime.yaml, keeping the previous config when
// the new one does not load. appbuild.RealtimeConfig satisfies it.
type realtimeReloader interface {
	Reload(ctx context.Context) error
}

// SetRealtime gives the app the realtime configuration holder whose file
// it watches. Call it before StartWatching. A free function because App's
// method count is pinned (plimsoll).
//
// Nil: r is rejected — an app without realtime simply does not call this.
func SetRealtime(a *App, r realtimeReloader) error {
	if r == nil {
		return errors.New("dataentry: SetRealtime requires a realtime config")
	}
	a.realtime = r
	return nil
}

// watchRealtime subscribes to realtime.yaml when a holder is set. A change
// event while a Configure save is in progress is ignored, as for
// data-entry.yaml.
//
// coverage-ignore-func: requires real filesystem events via the file watcher
func watchRealtime(a *App, sub config.Subscriber) error {
	if a.realtime == nil {
		return nil
	}
	stop, err := sub.Subscribe(context.Background(), realtimeFile, func() {
		if a.reloadPaused.Load() > 0 {
			return
		}
		_ = reloadRealtime(a)
	})
	if err != nil {
		return err
	}
	a.stopRealtimeWatch = stop
	return nil
}

// reloadRealtime re-reads realtime.yaml. An accepted file may change who
// reads which channel, so the live worker runs a full round (see
// [liveHub.policyChanged]). A rejected file keeps the previous
// config serving; browsers get a config-error frame naming realtime.yaml,
// and the returned error is a *configError. Configuration is not a secret,
// so the frame carries the problem itself, stripped of the host path.
func reloadRealtime(a *App) error {
	if a.realtime == nil {
		return nil
	}
	err := a.realtime.Reload(context.Background())
	if err == nil {
		slog.Info("realtime config reloaded")
		a.live.policyChanged()
		return nil
	}
	ce := &configError{public: publicConfigErrorMessage(err, a.paths.Root), err: err}
	slog.Warn("realtime config reload rejected; keeping previous config", "error", err)
	a.broker.broadcastConfigError(realtimeFile, ce)
	return ce
}
