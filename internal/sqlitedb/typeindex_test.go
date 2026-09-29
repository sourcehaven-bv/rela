package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// entityIndexes lists the names of the indexes on entities.
func entityIndexes(t *testing.T, raw *sql.DB) map[string]bool {
	t.Helper()
	rows, err := raw.Query(`SELECT name FROM sqlite_schema WHERE type = 'index' AND tbl_name = 'entities'`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		out[name] = true
	}
	require.NoError(t, rows.Err())
	return out
}

// TestMigrateToTypeIDFaceIndex pins the v7→v8 rung (TKT-KQXVF7): a v7
// database, which has the (type) index and not the (type, id, face) one,
// ends with only the latter, and a second open is harmless.
func TestMigrateToTypeIDFaceIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v7.db")
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	require.NoError(t, db.Close())

	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()
	for _, q := range []string{
		`DROP INDEX entities_type_id_face_idx`,
		`CREATE INDEX entities_type_idx ON entities(type)`,
		`PRAGMA user_version = 7`,
	} {
		_, err = raw.Exec(q)
		require.NoErrorf(t, err, "seed statement: %s", q)
	}

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	var version int
	require.NoError(t, raw.QueryRow("PRAGMA user_version").Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)

	got := entityIndexes(t, raw)
	require.True(t, got["entities_type_id_face_idx"], "the rung must leave the (type, id, face) index: %v", got)
	require.False(t, got["entities_type_idx"], "the rung must drop the (type) index it replaces: %v", got)
}

// TestFreshDatabaseHasTypeIDFaceIndex pins that schemaSQL creates the same
// index set the ladder arrives at.
func TestFreshDatabaseHasTypeIDFaceIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.db")
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	require.NoError(t, db.Close())

	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()
	got := entityIndexes(t, raw)
	require.True(t, got["entities_type_id_face_idx"], "%v", got)
	require.False(t, got["entities_type_idx"], "%v", got)
}
