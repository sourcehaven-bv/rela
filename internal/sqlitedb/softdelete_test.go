package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

func tableColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var cols []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		cols = append(cols, name)
	}
	require.NoError(t, rows.Err())
	return cols
}

// TestSoftDeleteTablesMatchLiveTables pins that the side tables hold every
// live column, in order. sqlitestore moves rows between the two with explicit
// column lists; a column added to a live table but not here would be dropped
// on restore.
func TestSoftDeleteTablesMatchLiveTables(t *testing.T) {
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: filepath.Join(t.TempDir(), "r.db")})
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	entities := tableColumns(t, db.DB(), "entities")
	assert.Equal(t, append(entities, "deleted_at", "deleted_by"), tableColumns(t, db.DB(), "marked_entities"))

	relations := tableColumns(t, db.DB(), "relations")
	assert.Equal(t, append([]string{"owner_id"}, relations...), tableColumns(t, db.DB(), "marked_relations"))
}

// TestMigrateToSoftDeleteTables pins that a v7 database gains the side tables.
func TestMigrateToSoftDeleteTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v7.db")
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	require.NoError(t, db.Close())

	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	for _, q := range []string{
		`DROP TABLE marked_entities`, `DROP TABLE marked_relations`, `PRAGMA user_version = 7`,
	} {
		_, err = raw.Exec(q)
		require.NoError(t, err, q)
	}
	require.NoError(t, raw.Close())

	db, err = sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	assert.NotEmpty(t, tableColumns(t, db.DB(), "marked_entities"))
	assert.NotEmpty(t, tableColumns(t, db.DB(), "marked_relations"))
	var v int
	require.NoError(t, db.DB().QueryRow(`PRAGMA user_version`).Scan(&v))
	assert.Equal(t, sqlitedb.SchemaVersion(), v)
}
