//go:build postgres

package appbuild

import (
	"log/slog"
	"os"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
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

// stateKVFor returns a database-backed [state.KV] sharing the store's pool, so
// the document render cache, user settings, the operator logo and scheduler
// bookkeeping are shared by every process serving this schema instead of living
// in each node's own .rela/ directory (TKT-VC27L3).
//
// That matters for the multi-process deployment docs/postgres-backend.md already
// documents: with an FSKV, an operator's logo upload lands on whichever node
// served the POST and every other node keeps serving the old one, with no error
// anywhere. It also means a schema-per-tenant deployment gets per-tenant state
// for free — the search_path that scopes entities scopes this table.
//
// Returns a genuinely nil interface for a non-pgstore store so the caller's
// nil-check falls back to the filesystem KV.
func stateKVFor(st store.Store) state.KV {
	raw := pgstore.StateStoreFor(st)
	if raw == nil {
		return nil
	}
	// pgstore stores whatever key it is handed; ValidatedKV applies the key
	// rules FSKV gets from RootedFS, so both backends accept exactly the same
	// keys. See state.ValidatedKV.
	kv, err := state.NewValidatedKV(raw)
	if err != nil {
		// Unreachable: raw is non-nil here. Fall back rather than fail
		// startup over an impossible case.
		slog.Warn("appbuild: could not wrap database state store; falling back "+
			"to the filesystem (state will be node-local)", "error", err)
		return nil
	}
	return kv
}
