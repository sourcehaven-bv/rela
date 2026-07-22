package pgstore

import "github.com/Sourcehaven-BV/rela/internal/store"

// VersionStore is the PostgreSQL content-versioning service: entity and relation
// history reads, synchronous version writes, and version purge. It is a SEPARATE
// concern from [Store] that merely shares the same database — like the in-DB
// search backend shares the pool. A store "just stores"; versioning is injected
// as its own service (or left nil in a deployment without it, e.g. the default
// filesystem build), rather than advertised as a pile of optional capabilities a
// consumer type-asserts off the store.
//
// It is constructed from the same [DBTX] the store uses (the shared pool). All
// its methods are pure functions over that handle — they hold no lifecycle state
// (no listener, no sweep, no subscriber fan-out); those stay on [Store]. The
// reconciliation sweep ([Store.StartVersionSweep]) is the store's own goroutine
// and does its inserts via the same free `insert*` helpers, not through this
// service, so the two never contend on shared receiver state.
//
// VersionStore satisfies the version capability interfaces in the store package
// ([store.HistoryReader], [store.VersionWriter], [store.RelationHistoryReader],
// [store.RelationVersionWriter], [store.VersionPurger],
// [store.RelationVersionPurger]) and their umbrella [store.VersionService]. The
// postgres composition root builds one and injects it wherever versioning is
// needed; the default build injects nil.
type VersionStore struct {
	db DBTX
}

// VersionStore returns a versioning service sharing this store's connection
// handle. The composition root calls this in the postgres recipe to obtain the
// service it injects; it reads the same pool the store queries, so the two never
// diverge. Returns a fresh lightweight wrapper each call (it holds only the
// shared handle). (The reconciliation sweep is separate — see StartVersionSweep.)
func (s *Store) VersionStore() *VersionStore {
	return &VersionStore{db: s.db}
}

// Compile-time checks that VersionStore provides the full version surface.
var (
	_ store.HistoryReader         = (*VersionStore)(nil)
	_ store.VersionWriter         = (*VersionStore)(nil)
	_ store.RelationHistoryReader = (*VersionStore)(nil)
	_ store.RelationVersionWriter = (*VersionStore)(nil)
	_ store.VersionPurger         = (*VersionStore)(nil)
	_ store.RelationVersionPurger = (*VersionStore)(nil)
	_ store.VersionService        = (*VersionStore)(nil)
)
