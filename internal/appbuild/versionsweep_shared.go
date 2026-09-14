//go:build postgres || sqlite

package appbuild

import (
	"log/slog"
	"os"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The backend-neutral half of the version wiring, shared by every build that
// has a versioning store.
//
// It lives in its own file rather than in either recipe because none of it is
// backend-specific: the projection comes from the metamodel, the cadence from
// the environment, and the two capabilities are named in store-package terms.
// Duplicating it per backend is how the recipes drift — the same reason
// prepare/assemble exist.

// metaProjectionProvider yields the current render-schema projection from the
// metamodel, for a version sweep to stamp on create/update versions. It reads
// the metamodel on each call so a metamodel reload is picked up on the next
// sweep tick.
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

// versionSweeper is the capability startVersionSweepIfSupported needs: a store
// that runs its own debounced reconciliation sweep to capture create/update
// versions.
//
// The signature names only store-package types, so any backend can satisfy it
// without importing pgstore (TKT-L3FNEN).
//
// An ALIAS for store.VersionSweeper, not a locally-declared interface: the
// consumer-side rule is about owning the minimum method list, and at one method
// there is nothing to narrow. The local name exists so the call site and the
// compile-time assertions in widen_assertions_postgres_test.go keep referring to
// one identifier.
type versionSweeper = store.VersionSweeper

// versionServiceProvider is the capability versionServiceFor needs: a store that
// can hand out a versioning service (history reads, version writes, purge)
// sharing its own connection.
//
// Returns the store.VersionService interface, so a backend satisfies this
// without importing pgstore (TKT-L3FNEN). An alias for the same reason as
// versionSweeper above.
type versionServiceProvider = store.VersionServiceProvider

// startVersionSweepIfSupported starts the store's reconciliation sweep. A store
// without the capability (should not happen in these builds) is left unswept.
// The sweep cadence is taken from sweepConfigFromEnv so a test/dev deployment
// can make create/update versions appear quickly (production uses the
// zero-value defaults: 5m interval/idle).
func startVersionSweepIfSupported(st store.Store, meta *metamodel.Metamodel) {
	if s, ok := st.(versionSweeper); ok {
		s.StartVersionSweep(metaProjectionProvider{meta: meta}, sweepConfigFromEnv())
	}
}

// versionServiceFor returns the store's versioning service (history reads,
// version writes, purge) sharing its connection. Returns a genuinely nil
// interface — never a typed nil — both for a store without the capability and
// for one whose handle came back nil, so nil-checks downstream behave
// correctly.
//
// The guard goes through nonNilCapability rather than a bare `== nil`, and that
// distinction is the whole point: VersionStore() returns an INTERFACE, so a
// backend returning a nil pointer yields a non-nil interface that a plain check
// cannot see. versionRecorderFor (appbuild.go) and startDataMigration would
// both pass their nil-checks and panic on first use — at write time, in
// production. See capability.go.
func versionServiceFor(st store.Store) store.VersionService {
	s, ok := st.(versionServiceProvider)
	if !ok {
		return nil
	}
	return nonNilCapability(s.VersionStore())
}

// sweepConfigFromEnv reads optional sweep-cadence overrides from the environment.
// All zero by default (→ the backend's 5m/5m/1h/500 defaults). Intended for
// e2e/dev where waiting minutes for create/update capture is impractical:
//
//	RELA_VERSION_SWEEP_INTERVAL / _IDLE / _MAX_STALENESS  (Go durations, e.g. 500ms)
//
// Unparseable values are ignored with a warning rather than failing boot — a
// misconfigured cadence must never take down the server; it just falls back to
// the default for that field.
func sweepConfigFromEnv() store.SweepConfig {
	dur := func(env string) time.Duration {
		v := os.Getenv(env)
		if v == "" {
			return 0
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			// G706 flags v as tainted because its analysis stops at the slog
			// call and cannot see the handler that encodes the record. The
			// message is a constant and v travels as a structured ATTRIBUTE,
			// which slog's handlers quote and escape — so an injected newline
			// cannot forge a second record. That is the same invariant
			// internal/dataentry pins in TestSlogTextHandlerEscapesNewlines.
			//
			// The value is worth echoing: this is an operator debugging their
			// own typo'd RELA_VERSION_SWEEP_* setting, and a warning that
			// withholds the rejected value is much harder to act on.
			//nolint:gosec // G706: constant message, user data as an escaped attribute
			slog.Warn("appbuild: ignoring invalid sweep duration",
				"env", env, "value", v, "error", err)
			return 0
		}
		return d
	}
	return store.SweepConfig{
		Interval:     dur("RELA_VERSION_SWEEP_INTERVAL"),
		Idle:         dur("RELA_VERSION_SWEEP_IDLE"),
		MaxStaleness: dur("RELA_VERSION_SWEEP_MAX_STALENESS"),
	}
}
