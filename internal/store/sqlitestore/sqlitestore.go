// Package sqlitestore implements [store.Store] on an embedded SQLite database.
//
// It BORROWS a handle opened by [sqlitedb], which owns the file: the pool, the
// single-writer lock and the schema ladder are all that package's. A rela
// database holds the entity graph AND the operator's config, so neither owns
// the other; this package's job is the graph.
//
// It targets the SINGLE-PROCESS deployment — the desktop app and a single
// rela-server — and sits between fsstore and pgstore: it gives up fsstore's
// git-diffable markdown files and pgstore's cross-process serialization, and in
// return provides indexed queries without a database server and real
// transaction rollback (DEC-LFSYNY).
//
// # Single writer, enforced
//
// Single-process is not an assumption this package hopes holds; sqlitedb.Open takes an
// exclusive lock on a sidecar file and refuses to start when another process
// holds it. That matters because rela's `unique:` enforcement is an
// untransacted scan in entitymanager: with two processes writing the same
// database there is no backstop, and the failure is silent.
//
// # Findings carried from the spike (TKT-TWIO11) — do not rediscover
//
// Each of these was measured, and each is cheap to reintroduce by accident:
//
//   - PRAGMAs go in the DSN, never db.Exec. A PRAGMA is per-connection, so
//     db.Exec configures whichever pooled connection served it and leaves every
//     later one at the default — while reading back correctly. Measured:
//     busy_timeout set that way failed at 0.00s instead of waiting 5s.
//   - Transactions use BEGIN IMMEDIATE. A deferred transaction that reads then
//     writes must upgrade its lock mid-flight, and the upgrade cannot wait, so
//     it returns SQLITE_BUSY regardless of busy_timeout.
//   - Never serialize by shrinking the pool. MaxOpenConns(1) deadlocks: Tx
//     pins the only connection while readers block waiting for one. That is
//     database/sql pool starvation, not a SQLite lock.
package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// timeFmt is the on-disk timestamp format for entity and relation rows.
// RFC3339Nano keeps the timezone, which is load-bearing: a naive timestamp
// parses back to a time that compares wrong against every consumer's clock,
// and store.Freshness is consumed by index-rebuild logic that does exactly
// that comparison.
const timeFmt = time.RFC3339Nano

// Store is a SQLite-backed [store.Store].
//
// Nil: never returned nil by sqlitedb.Open; a nil *Store is a programming error, not a
// supported "no store" value.
//
// The method count is the MANDATED store.Store interface, not accreted sprawl:
// all but one of the exported methods are the interface itself, and the
// remaining one is the constructor-adjacent JournalMode accessor. It ratchets
// with the interface, exactly as memstore's and
// fsstore's directives do — a "required interface" exception rather than a
// target to reduce. Anything ADDED beyond the interface should raise the
// question this line exists to ask.
//
// Two bumps since the first draft, both from review and both unexported
// helpers rather than new API: wrapping DeleteEntity in a transaction split it
// into an exported method plus a locked helper (the shape RenameEntity already
// had), and the schema guard carried the user_version check. The EXPORTED
// count has not moved, which is the number that actually measures coupling.
//
// The count came back DOWN when the connection was split out (TKT-S1EVV7):
// opening, PRAGMA verification and the migration ladder are Conn's, not the
// store's. Ratcheting the directive down with it is the point — see
// TKT-N0IKN9.
//
// Then back up by one for ProjectFiles, which is the exception the numbers
// exist to make you argue for rather than take silently. It is a one-line
// accessor over the pool the store already holds, and it has to be ON the
// store: the wiring site discovers the store-backed config layer by type
// assertion, so an accessor reachable only from Conn would make that assertion
// fail with no error anywhere. Still a net -2 against the pre-split count.
//
// The content-states surface (TKT-DOFYR1 / TKT-C1XUA8 / TKT-WAV8XP) is the
// latest interface growth: GetEntityState, DeleteEntityState and
// DeleteRelationState are store.Store methods, and each brought its own locked
// helper plus the family/relation scan helpers the family-wide semantics need.
// Interface-driven again, so the numbers move with store.Store rather than
// with this type.
//
//plimsoll:max-methods=51
//plimsoll:max-exported-methods=33
type Store struct {
	db *sql.DB

	// observers receive derived-state callbacks. Fixed at construction — see
	// WithObserver.
	observers []store.EntityObserver

	// writeMu serializes Tx bodies in-process, so a queued writer waits on a
	// mutex instead of burning its busy_timeout budget spinning on
	// SQLITE_BUSY. Only the root store takes it; a view never does.
	writeMu sync.Mutex

	subMu sync.Mutex
	subs  map[int]chan store.Event
	next  int

	// conn pins the transaction's connection on a view; nil on the root store,
	// where statements go to the pool.
	conn *sql.Conn

	// txPending buffers events raised inside a Tx until commit. Non-nil ALSO
	// marks "this view is inside a transaction" — the signal a nested Tx uses
	// to join rather than open a second one.
	txPending *pendingEvents

	// parent is the root store a view publishes through, so subscribers
	// registered on the root observe events emitted inside a Tx.
	parent *Store
}

// pendingEvents buffers post-commit work raised inside a transaction.
//
// It holds CALLBACKS rather than events because observers must be deferred
// too, not just the event fan-out: an observer that fires inside a rolled-back
// transaction leaves the search index holding a phantom entity, and nothing
// self-heals until a full reindex. One buffer keeps events and observer
// callbacks in the order they were raised.
type pendingEvents struct {
	mu    sync.Mutex
	notes []func(*Store)
}

// New builds a store on an already-opened database.
//
// It BORROWS the handle rather than owning it: the file — the pool, the
// single-writer lock, the schema — belongs to [sqlitedb.DB], and closing the
// store does not close the database. That inversion is deliberate. A rela
// database holds two unrelated things, the entity graph and the operator's
// config, and neither should own the other or the file they share; the caller
// opens once and hands the same handle to both.
//
// It also mirrors pgstore, whose New likewise takes an injected pool that the
// wiring site owns and closes.
//
// Nil: rejected — a nil database is a wiring mistake, and failing here beats a
// panic on the first query.
func New(db *sqlitedb.DB, options ...Option) (*Store, error) {
	if db == nil {
		return nil, errors.New("sqlitestore: nil database")
	}
	s := &Store{
		db:   db.DB(),
		subs: map[int]chan store.Event{},
	}
	for _, opt := range options {
		opt(s)
	}
	return s, nil
}

// --- execution seam -------------------------------------------------------

type querier interface {
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// q returns the execution target: the pinned transaction connection inside a
// Tx, the pool otherwise. One seam means one set of method bodies serves both.
func (s *Store) q() querier {
	if s.conn != nil {
		return s.conn
	}
	return s.db
}

// write executes a mutating statement against the current target.
func (s *Store) write(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return s.q().ExecContext(ctx, q, args...)
}

// --- Lifecycle ------------------------------------------------------------

// Close releases this store's subscribers.
//
// It does NOT close the database: the store borrows a handle owned by
// [sqlitedb.DB], and the wiring site that opened the file closes it. A
// transaction view closes nothing at all.
func (s *Store) Close() error {
	if s.parent != nil {
		return nil
	}
	s.subMu.Lock()
	for id, ch := range s.subs {
		delete(s.subs, id)
		close(ch)
	}
	s.subMu.Unlock()

	return nil
}
