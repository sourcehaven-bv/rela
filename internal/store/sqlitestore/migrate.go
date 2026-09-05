package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
)

// schemaVersion is the shape of the tables this binary expects. Bump it
// whenever schemaSQL changes shape, and append the step that carries an
// existing database forward to [migrations].
const schemaVersion = 2

// SchemaVersion reports the table shape this binary expects, so the CLI can
// show a real number rather than prose.
func SchemaVersion() int { return schemaVersion }

// migration is one rung of the ladder.
type migration struct {
	// to is the version this step produces. Index i must produce i+2 —
	// TestMigrationLadderIsWellFormed enforces it.
	to int
	// apply runs on a connection already inside a BEGIN IMMEDIATE that also
	// carries the version bump, so a failure rolls the whole step back rather
	// than leaving a half-migrated shape stamped as complete.
	apply func(context.Context, *sql.Conn) error
}

// sqlSteps adapts plain statements to [migration.apply].
func sqlSteps(stmts ...string) func(context.Context, *sql.Conn) error {
	return func(ctx context.Context, conn *sql.Conn) error {
		for _, stmt := range stmts {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return err
			}
		}
		return nil
	}
}

// migrations carries a database forward one version at a time. Index i holds
// the step from version i+1 to version i+2, so len(migrations)+1 must equal
// [schemaVersion].
//
// Forward-only, like pgstore's ladder: there is no down step, because a
// rollback that has to guess what a dropped column contained is a data-loss
// path dressed up as a safety feature.
//
// A step takes a connection rather than a list of statements because the steps
// that matter are rarely pure DDL — a backfill reads rows, transforms them and
// writes them back, which does not fit a []string. [sqlSteps] keeps the simple
// case short.
//
// Steps should still tolerate a re-run. A step runs once per database (the
// version bump commits with it), but that guarantee is worth exactly one
// crash: re-running IS the recovery path when a step commits and the process
// dies before the next one starts.
var migrations = []migration{
	{
		// v1 → v2: project_files, the table that lets a database carry the
		// operator-authored config (schema.yaml, scripts/, templates/) that
		// used to live only beside it on disk.
		to:    2,
		apply: sqlSteps(projectFilesDDL),
	},
}

// migrate stamps a fresh database, carries an older one forward, and refuses
// one written by a newer binary.
//
// fresh reports whether this database had no rela tables before schemaSQL ran;
// it MUST be measured before that, because schemaSQL is CREATE TABLE IF NOT
// EXISTS and afterwards the two cases are indistinguishable. A fresh database
// is already at the current shape, so it takes the stamp directly rather than
// replaying a ladder whose steps describe changes it never needed. Replaying
// it happens to work today only because the single step is IF NOT EXISTS; the
// first ordinary ALTER TABLE step would break every new install.
//
// The user_version marker matters more than it looks. schemaSQL being a silent
// NO-OP against an existing table of a DIFFERENT shape is exactly why: without
// a version an old database opens happily and then fails at the first query
// with "no such column", at runtime, on user data, with nothing pointing at
// the schema.
//
// Fail-loud on a newer version, matching pgstore.Migrate: a database from a
// newer binary is refused rather than opened and silently mis-read.
func (c *Conn) migrate(ctx context.Context, fresh bool) error {
	found, err := c.userVersion(ctx)
	if err != nil {
		return err
	}

	switch {
	case found == schemaVersion:
		return nil
	case found > schemaVersion:
		return fmt.Errorf(
			"sqlitestore: %s was written by a newer rela (schema version %d, "+
				"this binary understands %d); upgrade rela rather than "+
				"downgrading the database",
			c.opts.Path, found, schemaVersion)
	case fresh:
		return c.setUserVersion(ctx, schemaVersion)
	}

	// An unstamped database with tables in it predates the marker, which is
	// version 1 by definition — that was the only shape shipped before
	// stamping existed.
	if found == 0 {
		found = 1
	}
	for _, m := range migrations[found-1:] {
		if err := c.applyMigration(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// isFresh reports whether this database has no rela tables yet.
//
// Must be called BEFORE schemaSQL — see the migrate method below.
//
// Checked against entities rather than project_files: entities has existed
// since v1, so its absence means "nothing has ever been created here", whereas
// project_files is absent on every pre-v2 database as well.
//
// An unreadable sqlite_master is an error rather than a guess in either
// direction. Assuming "not fresh" would replay the ladder over a database that
// may already be current, which is only safe while every step happens to be
// idempotent.
func (c *Conn) isFresh(ctx context.Context) (bool, error) {
	var n int
	if err := c.db.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='entities'`,
	).Scan(&n); err != nil {
		return false, fmt.Errorf("sqlitestore: inspect schema: %w", err)
	}
	return n == 0, nil
}

// applyMigration runs one step and its version bump in a single transaction.
//
// BEGIN IMMEDIATE on a pinned connection, matching [Store.Tx] and for the same
// measured reason: a DEFERRED transaction that reads before it writes has to
// upgrade its lock mid-flight, and that upgrade returns SQLITE_BUSY regardless
// of busy_timeout. The current step is write-only, but a backfill is the shape
// a future step is most likely to take, and mid-migration on user data is the
// worst possible moment to discover the rule.
//
// The deferred ROLLBACK is load-bearing for the same reason it is in Tx: a
// connection returned to the pool with a transaction still open poisons every
// later use of it. It runs on WithoutCancel so a cancelled context still
// releases the transaction rather than abandoning it open.
func (c *Conn) applyMigration(ctx context.Context, m migration) error {
	fail := func(err error) error {
		return fmt.Errorf("sqlitestore: migrate to v%d: %w", m.to, err)
	}

	conn, err := c.db.Conn(ctx)
	if err != nil {
		return fail(err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
		_ = conn.Close()
	}()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fail(err)
	}
	if err := m.apply(ctx, conn); err != nil {
		return fail(err)
	}
	// PRAGMA user_version takes no bind parameter, so the value is
	// interpolated. It comes from the ladder above, never from input.
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.to)); err != nil {
		return fail(fmt.Errorf("stamp version: %w", err))
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fail(err)
	}
	committed = true
	return nil
}

// userVersion reads the schema version stamped on the database.
func (c *Conn) userVersion(ctx context.Context) (int, error) {
	var v int
	if err := c.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("sqlitestore: read user_version: %w", err)
	}
	return v, nil
}

// setUserVersion stamps the schema version on the database.
func (c *Conn) setUserVersion(ctx context.Context, v int) error {
	// Interpolated for the same reason as in applyMigration: PRAGMA takes no
	// bind parameter, and v is a package constant.
	if _, err := c.db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", v)); err != nil {
		return fmt.Errorf("sqlitestore: stamp user_version: %w", err)
	}
	return nil
}

// Status reports the version a database is stamped with and the version this
// binary expects, WITHOUT migrating it.
//
// Deliberately not implemented via [Connect]: connecting runs the ladder, so
// it could only ever report "already current" and `rela db status` would have
// nothing to say. This opens read-only and takes no single-writer lock, so it
// can answer while a server is running.
//
// A database file that does not exist reports version 0 rather than an error:
// "not created yet" is a meaningful answer to "what version is it". A
// zero-byte file — a crash artifact — reports 0 too, and the two are
// deliberately conflated, because both mean "there is no usable schema here",
// which is what the caller is asking.
func Status(ctx context.Context, path string) (found, want int, err error) {
	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		return 0, schemaVersion, nil
	}
	db, err := sql.Open("sqlite", readOnlyDSN(path))
	if err != nil {
		return 0, schemaVersion, fmt.Errorf("sqlitestore: open %s: %w", path, err)
	}
	defer func() { _ = db.Close() }()

	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&found); err != nil {
		return 0, schemaVersion, fmt.Errorf("sqlitestore: read user_version for %s: %w", path, err)
	}
	return found, schemaVersion, nil
}

// readOnlyDSN builds a read-only URI DSN for path.
//
// The `file:` scheme is REQUIRED to pass mode=ro, and it is also what makes
// escaping mandatory: in URI mode SQLite reads '#' as a fragment and '%' as a
// percent-escape, so a concatenated path containing either silently addresses
// a DIFFERENT file. Measured: a project at "…/a#b.db" opened (and, under this
// driver, CREATED) "…/a", read no schema from it and reported version 0 — a
// fabricated "database is behind" with a junk file left behind.
func readOnlyDSN(path string) string {
	u := url.URL{Scheme: "file", Opaque: (&url.URL{Path: path}).EscapedPath()}
	u.RawQuery = "mode=ro"
	return u.String()
}
