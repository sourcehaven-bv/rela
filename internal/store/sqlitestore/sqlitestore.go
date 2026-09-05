// Package sqlitestore implements [store.Store] on an embedded SQLite database
// via the pure-Go modernc.org/sqlite driver (no cgo).
//
// It targets the SINGLE-PROCESS deployment — the desktop app and a single
// rela-server — and sits between fsstore and pgstore: it gives up fsstore's
// git-diffable markdown files and pgstore's cross-process serialization, and in
// return provides indexed queries without a database server and real
// transaction rollback (DEC-LFSYNY).
//
// # Single writer, enforced
//
// Single-process is not an assumption this package hopes holds; [Open] takes an
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
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// timeFmt is the on-disk timestamp format. RFC3339Nano keeps the timezone,
// which is load-bearing: a naive timestamp parses back to a time that compares
// wrong against every consumer's clock, and store.Freshness is consumed by
// index-rebuild logic that does exactly that comparison.
const timeFmt = time.RFC3339Nano

// defaultBusyTimeout is how long a writer waits for the write lock before
// giving up. Generous on purpose — with the in-process write mutex below, a
// queued writer normally waits on the mutex rather than spending this budget,
// so reaching it means genuine contention worth waiting out.
const defaultBusyTimeout = 5 * time.Second

// defaultMaxOpenConns sizes the pool. Must be > 1: see the package doc on pool
// starvation.
const defaultMaxOpenConns = 8

// Options configures a store. The zero value is valid for every field except
// Path.
type Options struct {
	// Path is the database file. Required.
	Path string

	// BusyTimeout is how long a blocked writer waits. Zero uses
	// defaultBusyTimeout.
	BusyTimeout time.Duration

	// MaxOpenConns caps the connection pool. Values below 2 — the zero value
	// included — are raised to defaultMaxOpenConns, because a pool of one
	// deadlocks any Tx that runs concurrently with a read.
	MaxOpenConns int

	// AllowNonWAL permits opening a database where WAL could not be enabled.
	//
	// Default false, and that default is the point: WAL needs shared memory,
	// so it silently stays "delete" on most network and sync filesystems
	// (iCloud, Dropbox, SMB) — where SQLite is unsafe and the sidecar lock is
	// unreliable too. A desktop user who puts a project in iCloud otherwise
	// gets corruption with no diagnostic. Set this only for a deliberate,
	// understood exception.
	AllowNonWAL bool
}

// Store is a SQLite-backed [store.Store].
//
// Nil: never returned nil by [Open]; a nil *Store is a programming error, not a
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
// The content-states surface (TKT-DOFYR1 / TKT-C1XUA8 / TKT-WAV8XP) is the
// latest interface growth: GetEntityState, DeleteEntityState and
// DeleteRelationState are store.Store methods, and each brought its own locked
// helper plus the family/relation scan helpers the family-wide semantics need.
// Interface-driven again, so the numbers move with store.Store rather than
// with this type.
//
//plimsoll:max-methods=50
//plimsoll:max-exported-methods=32
type Store struct {
	db   *sql.DB
	opts Options

	// journalMode is what PRAGMA journal_mode actually reported, not what was
	// requested.
	journalMode string

	// lock is the sidecar single-writer lock; released by Close.
	lock *processLock

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

// New builds a store on an already-opened [Conn], TAKING OWNERSHIP of it:
// [Store.Close] closes the database and releases the single-writer lock, so
// the caller must not also close the Conn.
//
// Taking a connection rather than a path is what lets config living in this
// same database be read before the store exists — see the [Conn] doc for the
// ordering. It mirrors pgstore's New, which likewise takes an injected pool.
//
// Nil: rejected — a nil Conn is a wiring mistake, and failing here beats a
// panic on the first query.
func New(conn *Conn, options ...Option) (*Store, error) {
	if conn == nil {
		return nil, errors.New("sqlitestore: nil Conn")
	}
	s := &Store{
		db:          conn.db,
		opts:        conn.opts,
		journalMode: conn.journalMode,
		lock:        conn.lock,
		subs:        map[int]chan store.Event{},
	}
	for _, opt := range options {
		opt(s)
	}
	return s, nil
}

// Open connects to the database at opts.Path and builds a store on it.
//
// A convenience for callers that need nothing between the two steps;
// [Connect] plus [New] is the path for those that do.
//
// Nil: never returns a nil Store with a nil error.
func Open(opts Options, options ...Option) (*Store, error) {
	return OpenContext(context.Background(), opts, options...)
}

// OpenContext is [Open] with a caller-supplied context governing the
// startup work — the PRAGMA read-back, schema creation and migration.
//
// Separate from Open because the context bounds only opening: it does NOT
// govern the returned store, whose own methods each take their own. A caller
// that passed a request context here and expected it to cancel later writes
// would be wrong, so the two are kept visibly distinct.
//
// Nil: never returns a nil Store with a nil error.
func OpenContext(ctx context.Context, opts Options, options ...Option) (*Store, error) {
	conn, err := Connect(ctx, opts)
	if err != nil {
		return nil, err
	}
	s, err := New(conn, options...)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return s, nil
}

// dsn builds the connection string. Every PRAGMA is a DSN parameter so it
// applies to EVERY pooled connection — see the package doc.
func dsn(opts Options) string {
	return fmt.Sprintf(
		"%s?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)"+
			"&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)",
		opts.Path, opts.BusyTimeout.Milliseconds())
}

// JournalMode reports the journal mode actually in effect.
func (s *Store) JournalMode() string { return s.journalMode }

// schemaSQL is created unconditionally at Connect, and is the shape of a
// FRESH database. It is CREATE TABLE IF NOT EXISTS throughout, so it is a
// silent no-op against an existing table of a different shape — carrying an
// older database forward is migrate.go's job, and every change here needs a
// matching step there.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS entities (
	id          TEXT NOT NULL,
	-- face is the content-state coordinate (TKT-DOFYR1); '' is the DEFAULT
	-- state, so a faceless project stores exactly the rows it always did.
	--
	-- '' NOT NULL rather than NULL, matching pgstore: the face joins the
	-- primary key, and PK columns cannot be NULL. One convention everywhere —
	-- Go zero value, omitted frontmatter key, '' column.
	--
	-- The store only ever EQUALITY-MATCHES this value (see entity.Face), so
	-- one plain TEXT column suffices and keeps suffixing when multi-axis
	-- coordinates arrive: worlds compile to sets of concrete coordinates
	-- before they reach a store.
	face        TEXT NOT NULL DEFAULT '',
	type        TEXT NOT NULL,
	properties  TEXT NOT NULL DEFAULT '{}',
	content     TEXT NOT NULL DEFAULT '',
	updated_at  TEXT NOT NULL,
	PRIMARY KEY (id, face)
) STRICT;
CREATE INDEX IF NOT EXISTS entities_type_idx ON entities(type);
-- Entity IDs are case-insensitive IDENTITIES (BUG-3RCWNS): "abc" and "ABC"
-- cannot coexist. Enforced as a unique index on lower(id) rather than by
-- changing the column collation, exactly as pgstore does — the primary key
-- stays byte-exact so every id lookup keeps its semantics and index usage, and
-- casing is still PRESERVED on the row. Only the uniqueness rule widens.
--
-- The backends must agree on identity to stay substitutable: fsstore writes
-- "<id>.md" and so inherits the host filesystem's case folding, which would
-- silently drop one of the pair on macOS or Windows.
--
-- The face joins the key here too (pgstore's 0011 migration does the same):
-- states of ONE id legitimately share lower(id), so uniqueness is per
-- (lower(id), face). What the index does NOT catch — ('ABC','') alongside an
-- existing ('abc','draft') — is rejected by the write path's family probe
-- instead: a state requires ITS OWN default row, and 'abc' has none.
CREATE UNIQUE INDEX IF NOT EXISTS entities_id_lower_key ON entities(lower(id), face);

CREATE TABLE IF NOT EXISTS relations (
	from_id    TEXT NOT NULL,
	-- from_face is the state-specific TAIL (design doc §2.3). There is
	-- deliberately no to_face: heads stay entity-level, which is what makes
	-- cross-world dangling references impossible.
	from_face  TEXT NOT NULL DEFAULT '',
	rel_type   TEXT NOT NULL,
	to_id      TEXT NOT NULL,
	properties TEXT NOT NULL DEFAULT '{}',
	content    TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL,
	PRIMARY KEY (from_id, from_face, rel_type, to_id)
) STRICT;
CREATE INDEX IF NOT EXISTS relations_from_idx ON relations(from_id);
CREATE INDEX IF NOT EXISTS relations_to_idx   ON relations(to_id);

CREATE TABLE IF NOT EXISTS attachments (
	entity_id  TEXT NOT NULL,
	property   TEXT NOT NULL,
	file_name  TEXT NOT NULL,
	data       BLOB NOT NULL,
	size       INTEGER NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (entity_id, property, file_name)
) STRICT;
CREATE INDEX IF NOT EXISTS attachments_entity_idx ON attachments(entity_id);
` + projectFilesDDL + `
`

// projectFilesDDL carries the operator-authored config — schema.yaml,
// data-entry.yaml, acl.yaml, scripts/, templates/, custom/ — so a single
// database file can be a complete, shippable rela project rather than the data
// half of one.
//
// Flat path keys with no directory rows: listing is a prefix scan, which is
// all any consumer needs, and it keeps the two config backends agreeing about
// what a directory is (a filesystem has real ones; this has keys containing
// slashes). BLOB rather than TEXT because custom/ and apps/ carry fonts and
// images alongside the YAML.
//
// Shared between schemaSQL (fresh databases) and the v1→v2 migration
// (existing ones). One definition, not two copies: when duplicated DDL drifts,
// a fresh database and a migrated one end up with different shapes — precisely
// the failure the version-stamping apparatus exists to prevent, arriving by
// the one route it cannot detect.
const projectFilesDDL = `
CREATE TABLE IF NOT EXISTS project_files (
	path       TEXT PRIMARY KEY,
	content    BLOB NOT NULL,
	updated_at TEXT NOT NULL
) STRICT;`

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

// Close tears down the pool, closes every subscriber channel and releases the
// single-writer lock. A transaction view never closes the shared pool.
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

	err := s.db.Close()
	if lockErr := s.lock.release(); lockErr != nil && err == nil {
		err = lockErr
	}
	return err
}
