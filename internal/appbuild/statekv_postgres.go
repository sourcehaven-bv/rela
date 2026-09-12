//go:build postgres

package appbuild

import (
	"context"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

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
//
// The coupling is real but self-announcing: if state.KV gains a method,
// state.NewValidatedKV below stops accepting a rawStateStore and the build
// breaks HERE, at the call, with a clear message. That is the intended
// enforcement — by the compiler rather than by this comment.
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
