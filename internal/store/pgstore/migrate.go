package pgstore

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migration is one versioned SQL file.
type migration struct {
	version int
	name    string
	sql     string
}

// migrateAdvisoryLockKey is an arbitrary constant identifying the pgstore
// migration lock. pg_advisory_xact_lock serializes concurrent migrators (rela /
// rela-server / rela mcp may all call Migrate against the same database); the
// transaction-scoped lock is released automatically on commit or rollback.
const migrateAdvisoryLockKey = 0x52_45_4c_41 // "RELA"

// Migrate applies any unapplied migrations to the database in version order,
// idempotently and concurrency-safely. It is safe to call on every startup and
// from multiple processes at once: a transaction-scoped advisory lock
// serializes migrators, and already-applied migrations are skipped.
//
// The whole sequence runs in ONE transaction (PostgreSQL DDL is transactional),
// so a partial failure rolls back every change and the recorded schema_version
// never gets ahead of the applied DDL.
//
// The wiring layer calls this once at startup against the production pool;
// tests call it once per freshly-created schema.
func Migrate(ctx context.Context, db DBTX) error {
	migs, err := loadMigrations()
	if err != nil {
		return err
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	// Serialize concurrent migrators. The lock is held until this tx ends.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, migrateAdvisoryLockKey); err != nil {
		return fmt.Errorf("pgstore: acquire migration lock: %w", err)
	}

	// schema_version is a singleton: the CHECK + single-value PK forbid a second
	// row, so the version can never fork.
	if _, err = tx.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (
			id BOOLEAN PRIMARY KEY DEFAULT true CHECK (id),
			version INT NOT NULL
		)`); err != nil {
		return fmt.Errorf("pgstore: ensure schema_version: %w", err)
	}

	var current int
	var v *int
	if err = tx.QueryRow(ctx, `SELECT version FROM schema_version`).Scan(&v); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("pgstore: read schema_version: %w", err)
		}
	}
	if v != nil {
		current = *v
	}

	applied := current
	for _, m := range migs {
		if m.version <= current {
			continue
		}
		if _, err = tx.Exec(ctx, m.sql); err != nil {
			return fmt.Errorf("pgstore: migration %d (%s): %w", m.version, m.name, err)
		}
		applied = m.version
	}

	if applied != current {
		if _, err = tx.Exec(ctx,
			`INSERT INTO schema_version (id, version) VALUES (true, $1)
			 ON CONFLICT (id) DO UPDATE SET version = excluded.version`, applied); err != nil {
			return fmt.Errorf("pgstore: record schema_version: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// loadMigrations reads and parses the embedded migration files. File names
// must be "<version>_<name>.sql" with a zero-padded or plain integer version.
func loadMigrations() ([]migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("pgstore: read migrations dir: %w", err)
	}

	var migs []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		version, err := parseMigrationVersion(e.Name())
		if err != nil {
			return nil, err
		}
		data, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("pgstore: read migration %s: %w", e.Name(), err)
		}
		migs = append(migs, migration{version: version, name: e.Name(), sql: string(data)})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })
	return migs, nil
}

func parseMigrationVersion(name string) (int, error) {
	prefix, _, found := strings.Cut(name, "_")
	if !found {
		return 0, fmt.Errorf("pgstore: migration %q must be named <version>_<name>.sql", name)
	}
	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("pgstore: migration %q has non-numeric version prefix: %w", name, err)
	}
	return version, nil
}
