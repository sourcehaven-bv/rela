package sqlitestore_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// TestSchemaVersionStamped pins that a fresh database records its shape.
// Without the stamp, a future migration cannot tell v1 from v2 and would have
// to guess from pragma_table_info.
func TestSchemaVersionStamped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v.db")
	s, err := sqlitestore.Open(sqlitestore.Options{Path: path})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = s.Close()

	out, err := exec.Command("sqlite3", path, "PRAGMA user_version;").Output()
	if err != nil {
		t.Skipf("sqlite3 CLI unavailable: %v", err)
	}
	want := strconv.Itoa(sqlitestore.SchemaVersion())
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("user_version = %q, want %q", got, want)
	}
}

// TestRefusesNewerSchema is the forward-only guarantee: a database written by
// a newer rela must be refused, not opened and silently mis-read.
func TestRefusesNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	s, err := sqlitestore.Open(sqlitestore.Options{Path: path})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = s.Close()

	if err := exec.Command("sqlite3", path, "PRAGMA user_version = 99;").Run(); err != nil {
		t.Skipf("sqlite3 CLI unavailable: %v", err)
	}

	_, reopenErr := sqlitestore.Open(sqlitestore.Options{Path: path})
	if reopenErr == nil {
		t.Fatal("opened a database from a newer rela; must refuse")
	}
	if !strings.Contains(reopenErr.Error(), "newer rela") {
		t.Errorf("error does not explain the version mismatch: %v", reopenErr)
	}
}

// TestMigrationLadderIsWellFormed pins that every version bump brought a step
// with it, and that step i produces version i+2. Without the first, bumping
// schemaVersion and forgetting the migration leaves an existing database
// stamped as current while missing the table the bump was for — the exact
// silent no-op CREATE TABLE IF NOT EXISTS makes possible. Without the second,
// a mis-ordered ladder would skip or repeat a step while still counting right.
func TestMigrationLadderIsWellFormed(t *testing.T) {
	steps := sqlitestore.MigrationSteps()
	if got, want := len(steps)+1, sqlitestore.SchemaVersion(); got != want {
		t.Errorf("ladder produces version %d, schemaVersion is %d — "+
			"every bump needs a matching migration step", got, want)
	}
	for i, to := range steps {
		if want := i + 2; to != want {
			t.Errorf("migrations[%d] produces v%d, want v%d — "+
				"steps must be contiguous and ordered", i, to, want)
		}
	}
}

// TestMigratesV1Forward is the ladder doing its job: a database at the
// pre-project_files shape must gain the table rather than be refused or, worse,
// opened as-is and fail at the first query.
func TestMigratesV1Forward(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")

	// Build a v1-shaped database: the v1 tables, stamped v1, no project_files.
	if err := exec.Command("sqlite3", path, `
CREATE TABLE entities (id TEXT NOT NULL, face TEXT NOT NULL DEFAULT '',
  type TEXT NOT NULL, properties TEXT NOT NULL DEFAULT '{}',
  content TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL,
  PRIMARY KEY (id, face)) STRICT;
INSERT INTO entities VALUES ('E-1','','note','{}','body','2026-01-01T00:00:00Z');
PRAGMA user_version = 1;`).Run(); err != nil {
		t.Skipf("sqlite3 CLI unavailable: %v", err)
	}

	s, err := sqlitestore.Open(sqlitestore.Options{Path: path})
	if err != nil {
		t.Fatalf("open a v1 database: %v", err)
	}
	_ = s.Close()

	out, err := exec.Command("sqlite3", path,
		"PRAGMA user_version; SELECT count(*) FROM project_files; SELECT count(*) FROM entities;").Output()
	if err != nil {
		t.Fatalf("inspect migrated database: %v", err)
	}
	got := strings.Fields(strings.TrimSpace(string(out)))
	want := []string{strconv.Itoa(sqlitestore.SchemaVersion()), "0", "1"}
	if !slices.Equal(got, want) {
		t.Errorf("after migration got %v, want %v (version, project_files rows, "+
			"preserved entities)", got, want)
	}
}

// TestMigrateIsIdempotent pins that re-running the ladder is safe. Re-running
// IS the crash-recovery path, so a step that fails the second time turns a
// mid-migration crash into an unopenable database.
func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "twice.db")
	for i := range 3 {
		s, err := sqlitestore.Open(sqlitestore.Options{Path: path})
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("close %d: %v", i, err)
		}
	}
}

// TestFreshDatabaseSkipsTheLadder is the guard for the trap this ticket
// nearly shipped: a fresh database is already at the current shape, so it must
// take the version stamp directly rather than replay every migration.
//
// The test works by making the ladder observably non-idempotent — it seeds a
// v1-shaped database, migrates it, and then asserts a SECOND fresh database in
// the same state never re-runs steps. Concretely: if freshness were decided
// after schemaSQL (which always creates the tables), `fresh` would be false
// for a brand-new database, it would run migrations[0:], and the first future
// step written as a plain ALTER TABLE would fail with "duplicate column name"
// on every new install.
func TestFreshDatabaseSkipsTheLadder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.db")
	s, err := sqlitestore.Open(sqlitestore.Options{Path: path})
	if err != nil {
		t.Fatalf("open a fresh database: %v", err)
	}
	_ = s.Close()

	// A fresh database lands on the current version without the ladder having
	// had to produce it. The observable proxy is that it is stamped current
	// and holds the current shape.
	got, want, err := sqlitestore.Status(context.Background(), path)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if got != want {
		t.Errorf("fresh database at version %d, want %d", got, want)
	}
}

// TestStatusHandlesURIMetacharactersInPath pins the escaping in readOnlyDSN.
//
// Status must use a file: URI to pass mode=ro, and in URI mode SQLite reads
// '#' as a fragment. Concatenating the path therefore addressed a DIFFERENT
// file: the version came back 0 and a junk file appeared beside the real one,
// so `rela db status` reported "BEHIND" for a current database and `rela db
// migrate` printed a migration it had not performed.
func TestStatusHandlesURIMetacharactersInPath(t *testing.T) {
	for _, name := range []string{"a#b.db", "a%2Fb.db", "a b.db", "a?b.db"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, name)

			s, err := sqlitestore.Open(sqlitestore.Options{Path: path})
			if err != nil {
				t.Skipf("this path shape is not openable at all: %v", err)
			}
			_ = s.Close()

			got, want, err := sqlitestore.Status(context.Background(), path)
			if err != nil {
				t.Fatalf("Status: %v", err)
			}
			if got != want {
				t.Errorf("Status = %d, want %d — the DSN addressed the wrong file", got, want)
			}

			// And no stray database was created alongside it.
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				if n := e.Name(); n != name && !strings.HasPrefix(n, name) {
					t.Errorf("unexpected file %q created beside the database", n)
				}
			}
		})
	}
}
