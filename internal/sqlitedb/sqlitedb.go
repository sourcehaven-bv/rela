// Package sqlitedb owns the SQLite database FILE: opening it, verifying its
// PRAGMAs, holding the single-writer lock, and carrying its schema forward.
//
// It exists because a rela database file holds two unrelated things — the
// entity graph and the operator's config — and neither should own the other.
// Both are handed the opened handle:
//
//	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: p})  // owns the file
//	cfg     := configsql.New(db.DB())                          // reads config
//	st, err := sqlitestore.New(db)                             // reads the graph
//
// That ordering is also what config needs: the metamodel has to load before
// anything that consumes a metamodel, the store included. Previously the store
// owned opening, which made config a guest in a package whose job is the
// graph.
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
//   - Never serialize by shrinking the pool. MaxOpenConns(1) deadlocks: a
//     transaction pins the only connection while readers block waiting for one.
//     That is database/sql pool starvation, not a SQLite lock.
package sqlitedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// DB is an opened, verified SQLite database: the single-writer lock is
// held, the DSN PRAGMAs are confirmed to have reached more than one pooled
// connection, the schema exists and its version is stamped.
//
// It exists so config can be read BEFORE a store is built. A self-contained
// project keeps its schema.yaml and the rest of its operator-authored config
// in the same database as the data, and the metamodel must be loaded before
// anything that consumes a metamodel — the store included. Splitting the
// connection from the store makes that ordering expressible:
//
//	conn, err := sqlitestore.Connect(ctx, opts)   // database usable
//	meta, err := loadMetamodel(conn)              // config read from it
//	st, err := sqlitestore.New(conn, …)           // store built on it
//
// This mirrors pgstore, where [pgstore.New] takes an injected pool and the
// appbuild recipe owns and closes it. Same shape, second backend.
//
// Nil: never returned nil by [Open] alongside a nil error. A nil *DB is
// a programming error, not a supported "no database" value.
type DB struct {
	db          *sql.DB
	opts        Options
	journalMode string
	lock        *processLock
}

// Open opens (creating if absent) the database at opts.Path and returns a
// handle that is ready to query.
//
// The caller OWNS the result and must [DB.Close] it. Handing it to a store or
// a config loader does NOT transfer ownership — both borrow it, so whoever
// opened the file is who closes it.
//
// Errors are surfaced unchanged rather than wrapped: the actionable ones are
// "another process holds the single-writer lock" and "WAL could not be
// enabled because this is a network or sync filesystem", and burying either
// under an "open store" prefix hides the part the operator needs.
func Open(ctx context.Context, opts Options) (*DB, error) {
	if opts.Path == "" {
		return nil, errors.New("sqlitedb: Options.Path is required")
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
		return nil, fmt.Errorf("sqlitedb: open %s: %w", opts.Path, err)
	}
	db.SetMaxOpenConns(opts.MaxOpenConns)

	c := &DB{db: db, opts: opts, lock: lock}
	if err := c.init(ctx); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

// init verifies the connection settings actually took and creates the schema.
func (c *DB) init(ctx context.Context) error {
	if err := c.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&c.journalMode); err != nil {
		return fmt.Errorf("sqlitedb: read journal_mode: %w", err)
	}
	if c.journalMode != "wal" && !c.opts.AllowNonWAL {
		return fmt.Errorf(
			"sqlitedb: WAL could not be enabled (journal_mode=%q) for %s — "+
				"this usually means the file is on a network or file-sync "+
				"filesystem (iCloud, Dropbox, SMB), where SQLite is not safe. "+
				"Move the project to local storage, or set AllowNonWAL if you "+
				"understand the risk",
			c.journalMode, c.opts.Path)
	}

	if err := c.verifyBusyTimeout(ctx); err != nil {
		return err
	}

	// Freshness must be decided BEFORE schemaSQL runs. schemaSQL is CREATE
	// TABLE IF NOT EXISTS throughout, so afterwards a brand-new database and
	// a pre-existing one are indistinguishable — sqlite_master reports the
	// tables either way. Asking first is the only way to know which this is.
	fresh, err := c.isFresh(ctx)
	if err != nil {
		return err
	}
	if _, err := c.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("sqlitedb: create schema: %w", err)
	}
	return c.migrate(ctx, fresh)
}

// verifyBusyTimeout confirms the PRAGMA reached more than one connection.
//
// This exists because the failure it guards is SILENT: a per-connection PRAGMA
// applied to a single pooled connection reads back correctly while leaving
// every other connection at 0. Pinning two connections open simultaneously
// forces database/sql to open a genuinely different one, so a regression here
// (someone "simplifying" the DSN back to a db.Exec) fails at Open rather
// than as mysterious SQLITE_BUSY under load.
func (c *DB) verifyBusyTimeout(ctx context.Context) error {
	first, err := c.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlitedb: verify busy_timeout: %w", err)
	}
	defer first.Close()
	second, err := c.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlitedb: verify busy_timeout: %w", err)
	}
	defer second.Close()

	want := c.opts.BusyTimeout.Milliseconds()
	for i, conn := range []*sql.Conn{first, second} {
		var got int64
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&got); err != nil {
			return fmt.Errorf("sqlitedb: verify busy_timeout: %w", err)
		}
		if got != want {
			return fmt.Errorf(
				"sqlitedb: busy_timeout is %dms on connection %d, want %dms — "+
					"PRAGMAs must be set via DSN _pragma= parameters, not db.Exec",
				got, i, want)
		}
	}
	return nil
}

// JournalMode reports the journal mode actually in effect.
func (c *DB) JournalMode() string { return c.journalMode }

// DB exposes the pool, so config stored in this database can be read before a
// store exists and the two can share one connection.
//
// Returns a live handle, not a copy. Do not close it: close the [DB] that owns
// it, which is what tears down the pool and releases the single-writer lock.
func (c *DB) DB() *sql.DB { return c.db }

// Close tears down the pool and releases the single-writer lock.
//
// This is the ONLY thing that closes the database. A store or config loader
// built over this handle borrows it, so closing either of those leaves the
// file open — which is the point: they share it, and neither owns it.
func (c *DB) Close() error {
	err := c.db.Close()
	if lockErr := c.lock.release(); lockErr != nil && err == nil {
		err = lockErr
	}
	return err
}

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

// dsn builds the connection string. Every PRAGMA is a DSN parameter so it
// applies to EVERY pooled connection — see the package doc.
func dsn(opts Options) string {
	return fmt.Sprintf(
		"%s?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)"+
			"&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)",
		opts.Path, opts.BusyTimeout.Milliseconds())
}

// schemaSQL is created unconditionally at Open, and is the shape of a
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
