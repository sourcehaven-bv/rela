package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// realtimeFile is the realtime configuration file this app watches. The
// name is spelled here rather than imported: dataentry needs only to
// subscribe to it and report on it, never to parse it.
const realtimeFile = "realtime.yaml"

// realtimeReloader re-reads realtime.yaml, keeping the previous config when
// the new one does not load, and describes the config in force to the
// data-entry `channels:` check. appbuild.RealtimeConfig satisfies it.
type realtimeReloader interface {
	Reload(ctx context.Context) error
	// ChannelSchema is nil when realtime is off.
	ChannelSchema() dataentryconfig.ChannelSchema
}

// SetRealtime gives the app the realtime configuration holder whose file
// it watches, and checks data-entry.yaml's `channels:` section, kanban
// `busy:` card fields and list `busy:` columns against it: an entry that
// names what realtime.yaml does not declare fails the boot, as any other
// invalid data-entry.yaml does. Call it before StartWatching.
// A free function because App's method count is pinned (plimsoll).
//
// Nil: r is rejected — an app without realtime simply does not call this.
func SetRealtime(a *App, r realtimeReloader) error {
	if r == nil {
		return errors.New("dataentry: SetRealtime requires a realtime config")
	}
	if st := a.State(); st != nil {
		if err := checkChannelsPresentation(st.Cfg, r); err != nil {
			return err
		}
	}
	a.realtime = r
	return nil
}

// checkChannelsPresentation checks cfg's `channels:` section, kanban busy
// fields and list busy columns against the realtime config r holds now.
// It logs the warnings and returns the errors as one error naming
// data-entry.yaml. With r nil there is nothing to check against.
func checkChannelsPresentation(cfg *Config, r realtimeReloader) error {
	if r == nil {
		return nil
	}
	errs, warnings := dataentryconfig.ValidateChannels(cfg, r.ChannelSchema())
	for _, w := range warnings {
		slog.Warn("data-entry.yaml realtime presentation", "warning", w)
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid %s: %w", ConfigFile, &dataentryconfig.ConfigValidationError{Errors: errs})
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
// [liveHub.policyChanged]), and browsers get a refresh frame, so the SPA
// reloads the channel descriptor and labels. A rejected file keeps the
// previous config serving; browsers get a config-error frame naming
// realtime.yaml, and the returned error is a *configError. Configuration
// is not a secret, so the frame carries the problem itself, stripped of
// the host path.
//
// An accepted file also re-reads data-entry.yaml from disk and checks it
// against the new realtime config, so the order in which an operator saves
// the two files does not matter: a data-entry.yaml refused earlier because
// it named what realtime.yaml did not declare yet goes into force now. When
// data-entry.yaml does not load against the new realtime config, realtime.yaml
// stays in force (it is already applied) and browsers get a config-error frame
// naming data-entry.yaml with applied set to realtime.yaml, then refresh; the
// SPA ignores what the descriptor no longer declares.
//
// It holds a.reloadMu, as App.reloadConfig does, so the two
// check-and-publish paths never interleave.
func reloadRealtime(a *App) error {
	if a.realtime == nil {
		return nil
	}
	a.reloadMu.Lock()
	defer a.reloadMu.Unlock()
	err := a.realtime.Reload(context.Background())
	if err != nil {
		ce := &configError{public: publicConfigErrorMessage(err, a.paths.Root), err: err}
		slog.Warn("realtime config reload rejected; keeping previous config", "error", err)
		a.broker.broadcastConfigError(realtimeFile, ce, "")
		return ce
	}
	slog.Info("realtime config reloaded")
	a.live.policyChanged()
	if a.State() != nil {
		cfg, ce := readConfig(a)
		if ce != nil {
			slog.Warn("realtime.yaml applied; data-entry.yaml does not load against it", "error", ce.err)
			a.broker.broadcastConfigError(ConfigFile, ce, realtimeFile)
		} else {
			publishConfig(a, cfg)
		}
	}
	a.broker.broadcast("refresh")
	return nil
}
