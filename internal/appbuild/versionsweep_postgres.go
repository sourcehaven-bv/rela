//go:build postgres

package appbuild

import (
	"log/slog"
	"os"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// metaProjectionProvider yields the current render-schema projection from the
// metamodel, for the pgstore version sweep to stamp on create/update versions.
// It reads the metamodel on each call so a metamodel reload is picked up on the
// next sweep tick.
type metaProjectionProvider struct {
	meta *metamodel.Metamodel
}

func (p metaProjectionProvider) Projection() (hash string, projectionJSON []byte) {
	proj := p.meta.RenderProjection()
	b, err := proj.JSON()
	if err != nil {
		// Unreachable short of a runtime bug (RenderProjection is trivially
		// marshalable). Return an empty hash so the sweep tick skips capture
		// this round rather than stamping an empty projection.
		return "", nil
	}
	return proj.Hash(), b
}

// startVersionSweepIfSupported starts the pgstore reconciliation sweep. In the
// postgres build the store is a *pgstore.Store; a non-pgstore store (should not
// happen in this build) is left unswept. The sweep cadence is taken from
// sweepConfigFromEnv so a test/dev deployment can make create/update versions
// appear quickly (production uses the zero-value defaults: 5m interval/idle).
func startVersionSweepIfSupported(st store.Store, meta *metamodel.Metamodel) {
	if s, ok := st.(*pgstore.Store); ok {
		s.StartVersionSweep(metaProjectionProvider{meta: meta}, sweepConfigFromEnv())
	}
}

// sweepConfigFromEnv reads optional sweep-cadence overrides from the environment.
// All zero by default (→ pgstore's 5m/5m/1h/500 defaults). Intended for e2e/dev
// where waiting minutes for create/update capture is impractical:
//
//	RELA_VERSION_SWEEP_INTERVAL / _IDLE / _MAX_STALENESS  (Go durations, e.g. 500ms)
//
// Unparseable values are ignored with a warning rather than failing boot — a
// misconfigured cadence must never take down the server; it just falls back to
// the default for that field.
func sweepConfigFromEnv() pgstore.SweepConfig {
	dur := func(env string) time.Duration {
		v := os.Getenv(env)
		if v == "" {
			return 0
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			slog.Warn("appbuild: ignoring invalid sweep duration", "env", env, "value", v, "error", err)
			return 0
		}
		return d
	}
	return pgstore.SweepConfig{
		Interval:     dur("RELA_VERSION_SWEEP_INTERVAL"),
		Idle:         dur("RELA_VERSION_SWEEP_IDLE"),
		MaxStaleness: dur("RELA_VERSION_SWEEP_MAX_STALENESS"),
	}
}

// versionServiceFor returns the pgstore versioning service (history reads, version
// writes, purge) sharing the store's pool. Returns a genuinely nil interface for a
// non-pgstore store (should not happen in this build), so nil-checks downstream
// behave correctly.
func versionServiceFor(st store.Store) store.VersionService {
	if s, ok := st.(*pgstore.Store); ok {
		return s.VersionStore()
	}
	return nil
}
