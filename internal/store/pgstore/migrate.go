package pgstore

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migration is one versioned SQL file.
type migration struct {
	version int
	name    string
	sql     string
}

// Migrate applies any unapplied migrations to the database in version order,
// idempotently. It is safe to call on every startup: already-applied
// migrations are skipped. Each migration runs in its own transaction together
// with the schema_version bump, so a partial failure leaves the recorded
// version consistent with the applied DDL.
//
// The wiring layer calls this once at startup against the production pool;
// tests call it once per freshly-created schema.
func Migrate(ctx context.Context, db DBTX) error {
	migs, err := loadMigrations()
	if err != nil {
		return err
	}

	if _, err = db.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (version INT NOT NULL)`); err != nil {
		return fmt.Errorf("pgstore: ensure schema_version: %w", err)
	}

	current, err := currentVersion(ctx, db)
	if err != nil {
		return err
	}

	for _, m := range migs {
		if m.version <= current {
			continue
		}
		if err := applyMigration(ctx, db, m); err != nil {
			return fmt.Errorf("pgstore: migration %d (%s): %w", m.version, m.name, err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db DBTX, m migration) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if _, err = tx.Exec(ctx, m.sql); err != nil {
		return err
	}
	// schema_version holds a single row once seeded; use UPDATE-or-INSERT so the
	// table reflects the latest applied version without accumulating rows.
	tag, err := tx.Exec(ctx, `UPDATE schema_version SET version = $1`, m.version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO schema_version (version) VALUES ($1)`, m.version); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// currentVersion returns the highest applied migration version, or 0 if none.
func currentVersion(ctx context.Context, db DBTX) (int, error) {
	var v *int
	if err := db.QueryRow(ctx, `SELECT max(version) FROM schema_version`).Scan(&v); err != nil {
		return 0, fmt.Errorf("pgstore: read schema_version: %w", err)
	}
	if v == nil {
		return 0, nil
	}
	return *v, nil
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
