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

	// MaxOpenConns caps the connection pool. Zero uses defaultMaxOpenConns.
	// Values below 2 are raised to 2 — a pool of one deadlocks any Tx that
	// runs concurrently with a read.
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
// 26 of the exported methods are the interface itself, and the rest are the
// documented optional capabilities (HeaderReader) plus the constructor-adjacent
// accessors. It ratchets with the interface, exactly as memstore's and
// fsstore's directives do — a "required interface" exception rather than a
// target to reduce. Anything ADDED beyond the interface should raise the
// question this line exists to ask.
//
//plimsoll:max-methods=41
//plimsoll:max-exported-methods=29
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

// Open opens (creating if absent) the SQLite database at opts.Path.
//
// Nil: never returns a nil Store with a nil error.
func Open(opts Options, options ...Option) (*Store, error) {
	if opts.Path == "" {
		return nil, errors.New("sqlitestore: Options.Path is required")
	}
	if opts.BusyTimeout <= 0 {
		opts.BusyTimeout = defaultBusyTimeout
	}
	if opts.MaxOpenConns < 2 {
		opts.MaxOpenConns = defaultMaxOpenConns
	}

	// Take the single-writer lock BEFORE opening the database, so a second
	// process is refused before it can write anything.
	lock, err := acquireProcessLock(opts.Path)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn(opts))
	if err != nil {
		_ = lock.release()
		return nil, fmt.Errorf("sqlitestore: open %s: %w", opts.Path, err)
	}
	db.SetMaxOpenConns(opts.MaxOpenConns)

	s := &Store{db: db, opts: opts, lock: lock, subs: map[int]chan store.Event{}}
	for _, opt := range options {
		opt(s)
	}
	if err := s.init(); err != nil {
		_ = db.Close()
		_ = lock.release()
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

// init verifies the connection settings actually took and creates the schema.
func (s *Store) init() error {
	ctx := context.Background()

	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&s.journalMode); err != nil {
		return fmt.Errorf("sqlitestore: read journal_mode: %w", err)
	}
	if s.journalMode != "wal" && !s.opts.AllowNonWAL {
		return fmt.Errorf(
			"sqlitestore: WAL could not be enabled (journal_mode=%q) for %s — "+
				"this usually means the file is on a network or file-sync "+
				"filesystem (iCloud, Dropbox, SMB), where SQLite is not safe. "+
				"Move the project to local storage, or set AllowNonWAL if you "+
				"understand the risk",
			s.journalMode, s.opts.Path)
	}

	if err := s.verifyBusyTimeout(ctx); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("sqlitestore: create schema: %w", err)
	}
	return nil
}

// verifyBusyTimeout confirms the PRAGMA reached more than one connection.
//
// This exists because the failure it guards is SILENT: a per-connection PRAGMA
// applied to a single pooled connection reads back correctly while leaving
// every other connection at 0. Pinning two connections open simultaneously
// forces database/sql to open a genuinely different one, so a regression here
// (someone "simplifying" the DSN back to a db.Exec) fails at Open rather than
// as mysterious SQLITE_BUSY under load.
func (s *Store) verifyBusyTimeout(ctx context.Context) error {
	first, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlitestore: verify busy_timeout: %w", err)
	}
	defer first.Close()
	second, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlitestore: verify busy_timeout: %w", err)
	}
	defer second.Close()

	want := s.opts.BusyTimeout.Milliseconds()
	for i, c := range []*sql.Conn{first, second} {
		var got int64
		if err := c.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&got); err != nil {
			return fmt.Errorf("sqlitestore: verify busy_timeout: %w", err)
		}
		if got != want {
			return fmt.Errorf(
				"sqlitestore: busy_timeout is %dms on connection %d, want %dms — "+
					"PRAGMAs must be set via DSN _pragma= parameters, not db.Exec",
				got, i, want)
		}
	}
	return nil
}

// JournalMode reports the journal mode actually in effect.
func (s *Store) JournalMode() string { return s.journalMode }

const schemaSQL = `
CREATE TABLE IF NOT EXISTS entities (
	id          TEXT PRIMARY KEY,
	type        TEXT NOT NULL,
	properties  TEXT NOT NULL DEFAULT '{}',
	content     TEXT NOT NULL DEFAULT '',
	updated_at  TEXT NOT NULL
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
CREATE UNIQUE INDEX IF NOT EXISTS entities_id_lower_key ON entities(lower(id));

CREATE TABLE IF NOT EXISTS relations (
	from_id    TEXT NOT NULL,
	rel_type   TEXT NOT NULL,
	to_id      TEXT NOT NULL,
	properties TEXT NOT NULL DEFAULT '{}',
	content    TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL,
	PRIMARY KEY (from_id, rel_type, to_id)
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
`

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
