package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// editorColumns are the attribution columns the v6→v7 step adds (BUG-07DNNY).
var editorColumns = []string{"last_edited_by_user", "last_edited_by_tool"}

// seedV6 builds a database at the v6 shape: the current shape minus the
// attribution columns, stamped 6, with one entity and one relation that
// predate them.
func seedV6(t *testing.T, path string) {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	require.NoError(t, db.Close())

	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()
	for _, table := range []string{"entities", "relations"} {
		for _, column := range editorColumns {
			_, err = raw.Exec("ALTER TABLE " + table + " DROP COLUMN " + column)
			require.NoError(t, err)
		}
	}
	for _, q := range []string{
		`INSERT INTO entities (id, type, updated_at) VALUES ('FEAT-1', 'feature', '2026-01-01T00:00:00Z')`,
		`INSERT INTO entities (id, type, updated_at) VALUES ('FEAT-2', 'feature', '2026-01-01T00:00:00Z')`,
		`INSERT INTO relations (from_id, rel_type, to_id, updated_at, rel_record_id)
		 VALUES ('FEAT-1', 'depends-on', 'FEAT-2', '2026-01-01T00:00:00Z', 1)`,
		`PRAGMA user_version = 6`,
	} {
		_, err = raw.Exec(q)
		require.NoErrorf(t, err, "seed statement: %s", q)
	}
}

// TestMigrateToEditorColumns pins that a v6 database gains the attribution
// columns, that its existing rows read as "no recorded editor" (NULL, which the
// sweep credits to its system principal), and that re-opening is harmless.
func TestMigrateToEditorColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v6.db")
	seedV6(t, path)

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()

	var version int
	require.NoError(t, raw.QueryRow("PRAGMA user_version").Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)

	for _, table := range []string{"entities", "relations"} {
		var user, tool *string
		require.NoErrorf(t, raw.QueryRow(
			"SELECT "+editorColumns[0]+", "+editorColumns[1]+" FROM "+table).Scan(&user, &tool),
			"%s lacks the attribution columns after migrating", table)
		require.Nilf(t, user, "a pre-existing %s row must have no recorded editor", table)
		require.Nil(t, tool)
	}
}
