package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestMigrateToVersionTags pins the v15→v16 rung (TKT-VO6VG9): a v15
// database, which has no version_tags table, gains it with its foreign key
// to entity_versions, and a second open is harmless.
func TestMigrateToVersionTags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v15.db")
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	require.NoError(t, db.Close())

	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()
	for _, q := range []string{`DROP TABLE version_tags`, `PRAGMA user_version = 15`} {
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

	var table, from, to, onDelete string
	require.NoError(t, raw.QueryRow(
		`SELECT "table", "from", "to", on_delete FROM pragma_foreign_key_list('version_tags')`,
	).Scan(&table, &from, &to, &onDelete), "version_tags must exist with one foreign key")
	require.Equal(t, []string{"entity_versions", "vseq", "vseq", "RESTRICT"}, []string{table, from, to, onDelete})
}
