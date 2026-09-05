package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Conn is an opened, verified SQLite database: the single-writer lock is
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
// Nil: never returned nil by [Connect] alongside a nil error. A nil *Conn is
// a programming error, not a supported "no database" value.
type Conn struct {
	db          *sql.DB
	opts        Options
	journalMode string
	lock        *processLock
}

// Connect opens (creating if absent) the database at opts.Path and returns a
// handle that is ready to query.
//
// The caller OWNS the result and must [Conn.Close] it — unless it is handed
// to [New], which takes ownership so that closing the store closes the
// database exactly once.
//
// Errors are surfaced unchanged rather than wrapped: the actionable ones are
// "another process holds the single-writer lock" and "WAL could not be
// enabled because this is a network or sync filesystem", and burying either
// under an "open store" prefix hides the part the operator needs.
func Connect(ctx context.Context, opts Options) (*Conn, error) {
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

	c := &Conn{db: db, opts: opts, lock: lock}
	if err := c.init(ctx); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

// init verifies the connection settings actually took and creates the schema.
func (c *Conn) init(ctx context.Context) error {
	if err := c.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&c.journalMode); err != nil {
		return fmt.Errorf("sqlitestore: read journal_mode: %w", err)
	}
	if c.journalMode != "wal" && !c.opts.AllowNonWAL {
		return fmt.Errorf(
			"sqlitestore: WAL could not be enabled (journal_mode=%q) for %s — "+
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
		return fmt.Errorf("sqlitestore: create schema: %w", err)
	}
	return c.migrate(ctx, fresh)
}

// verifyBusyTimeout confirms the PRAGMA reached more than one connection.
//
// This exists because the failure it guards is SILENT: a per-connection PRAGMA
// applied to a single pooled connection reads back correctly while leaving
// every other connection at 0. Pinning two connections open simultaneously
// forces database/sql to open a genuinely different one, so a regression here
// (someone "simplifying" the DSN back to a db.Exec) fails at Connect rather
// than as mysterious SQLITE_BUSY under load.
func (c *Conn) verifyBusyTimeout(ctx context.Context) error {
	first, err := c.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlitestore: verify busy_timeout: %w", err)
	}
	defer first.Close()
	second, err := c.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlitestore: verify busy_timeout: %w", err)
	}
	defer second.Close()

	want := c.opts.BusyTimeout.Milliseconds()
	for i, conn := range []*sql.Conn{first, second} {
		var got int64
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&got); err != nil {
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
func (c *Conn) JournalMode() string { return c.journalMode }

// DB exposes the pool so config stored in this database can be read before a
// store exists. Returns a live handle, not a copy — do not close it; close
// the [Conn] (or the [Store] that took ownership of it) instead.
func (c *Conn) DB() *sql.DB { return c.db }

// Close tears down the pool and releases the single-writer lock.
//
// Closing a Conn that was handed to [New] is a double close: New takes
// ownership, so [Store.Close] already does this. Call this only for a Conn
// that never became a store.
func (c *Conn) Close() error {
	err := c.db.Close()
	if lockErr := c.lock.release(); lockErr != nil && err == nil {
		err = lockErr
	}
	return err
}
