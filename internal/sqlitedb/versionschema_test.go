package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// seedPreVersioning builds a database at the v3 shape — everything the
// versioning migration expects to find, and none of what it adds.
//
// Written out by hand rather than by opening at an older commit, which is the
// only way to test a migration at all: the point is to exercise the step
// against the shape it will actually meet in the wild, and the current binary
// can no longer produce that shape.
func seedPreVersioning(t *testing.T, path string, relations [][3]string) {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE entities (id TEXT NOT NULL, face TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL, properties TEXT NOT NULL DEFAULT '{}',
			content TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL,
			PRIMARY KEY (id, face)) STRICT`,
		`CREATE UNIQUE INDEX entities_id_lower_key ON entities(lower(id), face)`,
		// No rel_record_id: that column is exactly what the step adds.
		`CREATE TABLE relations (from_id TEXT NOT NULL, from_face TEXT NOT NULL DEFAULT '',
			rel_type TEXT NOT NULL, to_id TEXT NOT NULL, properties TEXT NOT NULL DEFAULT '{}',
			content TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL,
			PRIMARY KEY (from_id, from_face, rel_type, to_id)) STRICT`,
		`CREATE TABLE attachments (entity_id TEXT NOT NULL, property TEXT NOT NULL,
			file_name TEXT NOT NULL, data BLOB NOT NULL, size INTEGER NOT NULL,
			updated_at TEXT NOT NULL, PRIMARY KEY (entity_id, property, file_name)) STRICT`,
		`CREATE TABLE project_files (path TEXT PRIMARY KEY, content BLOB NOT NULL,
			updated_at TEXT NOT NULL) STRICT`,
		`CREATE TABLE state_kv (key TEXT PRIMARY KEY, value BLOB NOT NULL,
			updated_at TEXT NOT NULL) STRICT`,
		`INSERT INTO entities VALUES ('FEAT-1','','feature','{}','','2026-01-01T00:00:00Z')`,
		`INSERT INTO entities VALUES ('FEAT-2','','feature','{}','','2026-01-01T00:00:00Z')`,
		`PRAGMA user_version = 3`,
	}
	for _, q := range stmts {
		_, execErr := db.Exec(q)
		require.NoErrorf(t, execErr, "seed statement: %s", q)
	}
	for _, r := range relations {
		_, execErr := db.Exec(
			`INSERT INTO relations (from_id, rel_type, to_id, updated_at)
			 VALUES (?, ?, ?, '2026-01-01T00:00:00Z')`, r[0], r[1], r[2])
		require.NoError(t, execErr)
	}
}

// relRecordIDs reads the lineage ids off the relations table, ordered so the
// assertions can compare them directly.
func relRecordIDs(t *testing.T, path string) []int64 {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`SELECT rel_record_id FROM relations ORDER BY rel_record_id`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var out []int64
	for rows.Next() {
		var id int64
		require.NoError(t, rows.Scan(&id))
		out = append(out, id)
	}
	require.NoError(t, rows.Err())
	return out
}

// TestMigrateToVersioningBackfillsDistinctLineageIDs is the one assertion the
// v3→v4 step exists to earn.
//
// SQLite's ALTER TABLE ADD COLUMN takes only a CONSTANT default, so every
// pre-existing relation lands at the 0 sentinel. rel_record_id is what
// separates two relations' histories, so leaving them there would merge every
// relation in the database into a single lineage — a corruption that only
// surfaces the first time somebody reads a relation's history, long after the
// migration is forgotten.
func TestMigrateToVersioningBackfillsDistinctLineageIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pre-versioning.db")
	seedPreVersioning(t, path, [][3]string{
		{"FEAT-1", "depends-on", "FEAT-2"},
		{"FEAT-2", "depends-on", "FEAT-1"},
		{"FEAT-1", "relates-to", "FEAT-2"},
	})

	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err, "a pre-versioning database must migrate, not be refused")
	require.NoError(t, db.Close())

	ids := relRecordIDs(t, path)
	require.Len(t, ids, 3, "the migration must not drop relations")
	seen := map[int64]bool{}
	for _, id := range ids {
		require.NotZero(t, id, "a relation left at the 0 sentinel shares a lineage with every other")
		require.Falsef(t, seen[id], "lineage id %d assigned twice — two histories would merge", id)
		seen[id] = true
	}
}

// TestMigrateToVersioningIsIdempotent pins that re-opening a migrated database
// is safe. Re-running IS the crash-recovery path, so a step that fails the
// second time turns a mid-migration crash into an unopenable database — and
// this step's ALTER TABLE is the single statement in the versioning schema
// that would fail that way, which is why it is fenced off from the shared DDL.
func TestMigrateToVersioningIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "twice.db")
	seedPreVersioning(t, path, [][3]string{{"FEAT-1", "depends-on", "FEAT-2"}})

	var first []int64
	for i := range 3 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())

		ids := relRecordIDs(t, path)
		if i == 0 {
			first = ids
			continue
		}
		require.Equal(t, first, ids,
			"a re-open reassigned lineage ids; history would fork at every restart")
	}
}

// TestFreshDatabaseHasTheVersioningSchema pins that the fresh path and the
// migrated path agree on the shape.
//
// A fresh database skips the ladder entirely (TestFreshDatabaseSkipsTheLadder),
// so the versioning tables reach it only via schemaSQL. That is the drift this
// checks for: the two paths share versionSchemaSQL, but rel_record_id is
// deliberately NOT shared — the fresh path gets it from the relations CREATE
// TABLE and the migration from its own ALTER — and a column present on one
// path and absent on the other is precisely the failure the version stamp
// cannot detect.
func TestFreshDatabaseHasTheVersioningSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.db")
	fresh, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	require.NoError(t, fresh.Close())

	migratedPath := filepath.Join(t.TempDir(), "migrated.db")
	seedPreVersioning(t, migratedPath, nil)
	migrated, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: migratedPath})
	require.NoError(t, err)
	require.NoError(t, migrated.Close())

	require.Equal(t, tableShapes(t, path), tableShapes(t, migratedPath),
		"a fresh database and a migrated one must end up at the same shape")
}

// tableShapes reports the database's shape: every table's columns with their
// type/nullability/default, plus every index.
//
// sqlite_master.sql is deliberately NOT compared: it carries the literal
// CREATE text, so the fresh path's inline rel_record_id and the migration's
// ALTER-added one differ as strings while describing the same column.
// Comparing the resolved metadata instead asks the question that matters —
// "do these two databases behave the same" — rather than "were they typed the
// same way".
//
// Indexes are included because they are the half a column-only comparison
// misses: a CREATE INDEX omitted from the shared DDL would leave the migrated
// database correct but unindexed, which is invisible until the sweep's settle
// filter degrades to a full scan on a real project.
func tableShapes(t *testing.T, path string) map[string][]string {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	out := map[string][]string{}

	collect(t, db, "table:",
		`SELECT m.name, p.name || ' ' || p.type || ' notnull=' || p.[notnull] ||
		        ' default=' || COALESCE(p.dflt_value, '<none>')
		 FROM sqlite_master m
		 JOIN pragma_table_info(m.name) p
		 WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'
		 ORDER BY m.name, p.name`, out)

	collect(t, db, "index:",
		`SELECT COALESCE(tbl_name, ''), name FROM sqlite_master
		 WHERE type = 'index' AND name NOT LIKE 'sqlite_%'
		 ORDER BY tbl_name, name`, out)

	require.NotEmpty(t, out)
	return out
}

// collect runs a two-column (group, value) query into out under a key prefix.
func collect(t *testing.T, db *sql.DB, prefix, query string, out map[string][]string) {
	t.Helper()

	rows, err := db.Query(query)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var group, value string
		require.NoError(t, rows.Scan(&group, &value))
		out[prefix+group] = append(out[prefix+group], value)
	}
	require.NoError(t, rows.Err())
}
