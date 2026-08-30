package pgstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// maxStateValueBytes caps a single state value. The two large writers are the
// operator logo (an uploaded image) and a cached document render (HTML), so the
// ceiling is generous — but unbounded is wrong for a table every node reads:
// one pathological render would be fetched whole by every process on every
// cache hit. A value over the cap is refused at Put rather than truncated,
// since a silently truncated cached render would be served as if valid.
const maxStateValueBytes = 32 << 20 // 32 MiB

// StateKV is the PostgreSQL-backed durable key/value store. It structurally
// satisfies internal/state.KV; the interface is deliberately NOT named here,
// because a store backend may not depend on an application-level package (see
// .go-arch-lint.yml — pgstore's allowed deps are store-layer only). The wiring
// site binds the two.
//
// It exists because the filesystem backend is node-local. docs/postgres-backend.md
// documents running several rela-server processes against one database; under
// that topology an FSKV render cache is per-node (N nodes, up to N renders of
// the same document) and, worse, an operator's uploaded logo lands on whichever
// node served the POST while the others keep serving the old one — a visible
// bug with no error message. Putting this state in the database that is already
// the source of truth for entities and attachments fixes both.
//
// Rows live in the store's schema, so a schema-per-tenant deployment gets
// per-tenant state for free: the same search_path that scopes entities scopes
// this table.
//
// # Key policy lives with the caller
//
// This type stores whatever key it is given. Key VALIDATION (rejecting
// traversal, absolute paths, Windows-hostile names) is the state package's
// contract, enforced by state.ValidatedKV wrapping this — one implementation of
// those rules rather than a copy here that could drift from the filesystem
// backend's.
type StateKV struct {
	db DBTX
}

// NewStateKV constructs a StateKV over the given handle. db is typically the
// pool; state writes deliberately do NOT participate in a store.Tx (a document
// render is slow, and CLAUDE.md forbids slow I/O inside a Tx callback).
func NewStateKV(db DBTX) (*StateKV, error) {
	if db == nil {
		return nil, errors.New("pgstore: NewStateKV requires a database handle")
	}
	return &StateKV{db: db}, nil
}

// StateStoreFor returns a state store sharing st's connection handle, or nil if
// st is not a pgstore. The composition root calls this to obtain the service it
// injects, so the state table is read through the same pool the store queries.
//
// A package-level function rather than a method on [Store] on purpose. Store
// carries a pinned plimsoll load line and an explicit warning against adding
// capability accessors back onto it — every one re-invites the
// type-assert-a-capability-off-the-store pattern that the version refactor
// removed. Taking store.Store here keeps the wiring identical for the caller
// while leaving Store's method set untouched.
//
// The return type stays CONCRETE, deliberately (TKT-L3FNEN). Returning
// state.KV would read as the tidier decoupling, but pgstore must not import
// internal/state — arch-lint forbids a store depending on an application
// package, and that rule is what keeps key validation the state package's job
// (see the rawStateStore interface in appbuild, which does the widening on the
// consumer side where it belongs).
func StateStoreFor(st store.Store) *StateKV {
	s, ok := st.(*Store)
	if !ok {
		return nil
	}
	return &StateKV{db: s.db}
}

// Get returns the value at key, or an error satisfying os.IsNotExist when the
// key is absent. That specific error shape is load-bearing: the document cache
// tells "no cached render" from "the backend is broken" by testing it, so a
// bare not-found error would turn every cache miss into a hard failure.
func (s *StateKV) Get(ctx context.Context, key string) ([]byte, error) {
	var value []byte
	err := s.db.QueryRow(ctx, `SELECT value FROM state_kv WHERE key = $1`, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &os.PathError{Op: "get", Path: key, Err: os.ErrNotExist}
	}
	if err != nil {
		return nil, fmt.Errorf("pgstore: read state %q: %w", key, err)
	}
	// A NULL-free BYTEA scan can still yield nil for a zero-length value; the
	// contract distinguishes "empty" from "missing", so normalize to non-nil.
	if value == nil {
		value = []byte{}
	}
	return value, nil
}

// Put writes data at key, replacing any existing value.
func (s *StateKV) Put(ctx context.Context, key string, data []byte) error {
	if len(data) > maxStateValueBytes {
		return fmt.Errorf("pgstore: state value for %q is %d bytes, over the %d-byte limit",
			key, len(data), maxStateValueBytes)
	}
	// Copy defensively: pgx retains the slice for the duration of the call, and
	// a caller reusing its buffer must not be able to change what was stored.
	stored := bytes.Clone(data)
	if stored == nil {
		stored = []byte{}
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO state_kv (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		key, stored)
	if err != nil {
		return fmt.Errorf("pgstore: write state %q: %w", key, err)
	}
	return nil
}

// Delete removes the value at key. Deleting a missing key is NOT an error —
// callers (logoStore.Delete) clear optional state unconditionally and document
// themselves as idempotent.
func (s *StateKV) Delete(ctx context.Context, key string) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM state_kv WHERE key = $1`, key); err != nil {
		return fmt.Errorf("pgstore: delete state %q: %w", key, err)
	}
	return nil
}
