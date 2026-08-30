//go:build postgres

package appbuild

import (
	"context"
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

// versionSweeper is the capability startVersionSweepIfSupported needs: a store
// that runs its own debounced reconciliation sweep to capture create/update
// versions.
//
// The signature names only store-package types, so any backend can satisfy it
// without importing pgstore (TKT-L3FNEN). store.VersionSweeper says the same
// thing; this stays declared here because CLAUDE.md asks consumers to name the
// minimum interface they use at the call site.
type versionSweeper = store.VersionSweeper

// startVersionSweepIfSupported starts the store's reconciliation sweep. A store
// without the capability (should not happen in this build) is left unswept. The
// sweep cadence is taken from sweepConfigFromEnv so a test/dev deployment can
// make create/update versions appear quickly (production uses the zero-value
// defaults: 5m interval/idle).
func startVersionSweepIfSupported(st store.Store, meta *metamodel.Metamodel) {
	if s, ok := st.(versionSweeper); ok {
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

// versionServiceProvider is the capability versionServiceFor needs: a store that
// can hand out a versioning service (history reads, version writes, purge)
// sharing its own pool.
//
// Returns the store.VersionService interface, so a backend satisfies this
// without importing pgstore (TKT-L3FNEN).
type versionServiceProvider = store.VersionServiceProvider

// versionServiceFor returns the store's versioning service (history reads,
// version writes, purge) sharing its pool. Returns a genuinely nil interface —
// never a typed nil — both for a store without the capability and for one whose
// handle came back nil, so nil-checks downstream behave correctly.
//
// The explicit nil-pointer guard is load-bearing, and it became necessary when
// discovery widened from a concrete type to an interface. Asserting
// st.(*pgstore.Store) bounded the reachable implementations to exactly one,
// whose VersionStore() is unconditionally non-nil; an interface admits any
// implementation, including one that returns a nil pointer on a partial-init
// path. Boxing that into store.VersionService yields a NON-nil interface, so
// versionRecorderFor (appbuild.go) and startDataMigration would both pass their
// nil-checks and then panic on first use — at write time, in production.
func versionServiceFor(st store.Store) store.VersionService {
	s, ok := st.(versionServiceProvider)
	if !ok {
		return nil
	}
	vs := s.VersionStore()
	if vs == nil {
		return nil
	}
	return vs
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
//
// NOT widened by TKT-415WA7, unlike the three resolvers above: discovery still
// goes through pgstore.StateStoreFor, which type-asserts *pgstore.Store
// internally. So a second backend gets version sweeps, user state and derived
// stateKVFor returns a database-backed [state.KV] sharing the store's pool, so
// the document render cache, user settings, the operator logo and scheduler
// bookkeeping are shared by every process serving this schema instead of living
// in each node's own .rela/ directory (TKT-VC27L3).
//
// That matters for the multi-process deployment docs/postgres-backend.md already
// documents: with an FSKV, an operator's logo upload lands on whichever node
// served the POST and every other node keeps serving the old one, with no error
// anywhere.
//
// Discovery is by INTERFACE (TKT-L3FNEN), via rawStateStoreFor below.
//
// Returns a genuinely nil interface for a store without the capability, so the
// caller's nil-check falls back to the filesystem KV.
func stateKVFor(st store.Store) state.KV {
	raw := rawStateStoreFor(st)
	if raw == nil {
		// A capability that hands back nothing is not a capability. Guarding
		// here keeps a typed nil from being boxed into a non-nil state.KV,
		// which would pass every downstream nil-check and fail at first use.
		return nil
	}
	// The backend stores whatever key it is handed; ValidatedKV applies the key
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

// rawStateStore is the minimum a backend must offer to provide shared state: a
// key/value handle over its own connection.
//
// It restates state.KV's three methods structurally rather than naming the
// interface, and that is forced rather than stylistic. A store must not import
// internal/state (arch-lint: a store may not depend on an application package),
// which is the rule that keeps key validation the state package's job — so a
// backend cannot declare it returns a state.KV even though it satisfies one.
// Matching structurally lets the wiring site accept any such handle and wrap it
// in state.ValidatedKV, which is where the key rules are applied.
type rawStateStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, data []byte) error
	Delete(ctx context.Context, key string) error
}

// rawStateStoreFor discovers a backend's shared state handle.
//
// It calls pgstore.StateStoreFor rather than asserting a method on the store,
// because obtaining the handle must NOT become a Store method: pgstore.Store
// carries a pinned plimsoll line and an explicit warning that a further
// capability accessor must not raise it. The package function is that
// warning's answer, and this keeps the wiring honest about it — the *return*
// is now an interface, so nothing downstream names a pgstore type.
//
// Nil: returns a genuinely nil interface, never a typed nil, so the caller's
// fallback to the filesystem KV engages.
func rawStateStoreFor(st store.Store) rawStateStore {
	kv := pgstore.StateStoreFor(st)
	if kv == nil {
		return nil
	}
	return kv
}
