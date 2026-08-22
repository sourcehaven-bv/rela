package appbuild

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// startDataMigration wires the data-migration gate and GC sweep
// (TKT-0C57FS) for one assembled store. Build-agnostic: every backend gets
// the gate; the synchronous version capture inside GC deletions simply
// no-ops where the versioning service is nil.
//
// The gate evaluates once per process start (and again inside the `rela
// migrate` commands). There is no server-side metamodel hot-reload today —
// a schema.yaml edit under a running server is picked up on restart, and the
// gate verdict ages exactly as the served metamodel does, so the two can
// never disagree. If live metamodel reload lands, the reload path must call
// gate.Evaluate with the new metamodel (the Gate is built for it: verdicts
// publish via atomic.Pointer and the sweep reads the latest one each tick).
//
// Gate failures degrade to a warning: the gate must never fail boot
// (DEC-HWZHA keeps writes soft either way). The returned stop function
// terminates the GC sweep goroutine; Services.Close calls it — a
// per-assembled resource, never shared across tenants.
func startDataMigration(
	stateKV state.KV, meta *metamodel.Metamodel, st store.Store,
	aud audit.Audit, versions store.VersionService,
) (stop func()) {
	gate, err := datamigration.NewGate(stateKV)
	if err != nil {
		slog.Warn("datamigration: gate not started", "error", err)
		return func() {}
	}
	if v, evalErr := gate.Evaluate(context.Background(), meta); evalErr != nil {
		slog.Warn("datamigration: gate evaluation failed", "error", evalErr)
	} else if v.Status != datamigration.StatusInSync {
		slog.Info("datamigration: " + v.Describe())
	}

	// Kill switch fails SAFE: only unset or an explicit enable value keeps
	// the (data-deleting) sweep running — a typo'd "of"/"false"/"0" must
	// disable, not silently enable.
	switch os.Getenv("RELA_DATA_GC") {
	case "", "on", "1", "true":
	default:
		slog.Info("datamigration: gc sweep disabled via RELA_DATA_GC")
		return func() {}
	}
	var capture datamigration.VersionCapture
	if versions != nil {
		capture = versions
	}
	gc, err := datamigration.NewGC(datamigration.GCDeps{
		Store:    st,
		Meta:     func() *metamodel.Metamodel { return meta },
		State:    stateKV,
		Audit:    aud,
		Verdicts: gate,
		Versions: capture,
		Grace:    envDuration("RELA_DATA_GC_GRACE", 0), // 0 → datamigration.DefaultGrace
	})
	if err != nil {
		slog.Warn("datamigration: gc sweep not started", "error", err)
		return func() {}
	}

	// The first tick fires after one full interval, DELIBERATELY: services
	// are also assembled by short-lived CLI commands, and an immediate tick
	// would make every `rela list` implicitly delete expired drift. A
	// deployment whose server never lives a full interval runs
	// `rela migrate gc` explicitly (or via a scheduler task).
	interval := envDuration("RELA_DATA_GC_INTERVAL", time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				res, err := gc.Tick(ctx, true)
				switch {
				case err != nil:
					slog.Warn("datamigration: gc tick failed", "error", err)
				case res.Skipped != "":
					slog.Debug("datamigration: gc tick skipped", "reason", res.Skipped)
				case len(res.Deleted) > 0:
					slog.Info("datamigration: gc removed expired drift", "entries", len(res.Deleted))
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

// envDuration reads a Go duration from the environment, falling back on
// absence or a parse error (misconfigured cadence must never fail boot).
func envDuration(env string, fallback time.Duration) time.Duration {
	v := os.Getenv(env)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		// Value deliberately not logged (operator env is not a trusted log input).
		slog.Warn("datamigration: ignoring invalid duration", "env", env)
		return fallback
	}
	return d
}
