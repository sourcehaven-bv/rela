package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// originColumns are the provenance columns the v8→v9 step adds (BUG-YC5Z07).
var originColumns = []string{
	"origin_kind", "origin_source", "origin_source_face", "origin_source_type", "origin_definition",
}

// TestMigrateToOriginColumns pins that a v8 database gains the origin columns
// on entities, that existing rows read as a direct edit (all NULL), and that
// running the step again is harmless.
func TestMigrateToOriginColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v8.db")
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	require.NoError(t, db.Close())

	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()
	for _, column := range originColumns {
		_, err = raw.Exec("ALTER TABLE entities DROP COLUMN " + column)
		require.NoError(t, err)
	}
	_, err = raw.Exec(`INSERT INTO entities (id, type, updated_at)
		VALUES ('FEAT-1', 'feature', '2026-01-01T00:00:00.000000000Z')`)
	require.NoError(t, err)

	// Stamping v8 before each open makes the second open run the step again
	// over columns that now exist.
	for i := range 2 {
		_, err = raw.Exec(`PRAGMA user_version = 8`)
		require.NoError(t, err)
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	for _, column := range originColumns {
		var v *string
		require.NoErrorf(t, raw.QueryRow("SELECT "+column+" FROM entities").Scan(&v),
			"entities lacks %s after migrating", column)
		require.Nilf(t, v, "a pre-existing row must read as a direct edit (%s)", column)
	}
}
