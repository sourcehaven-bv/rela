package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// contentHashTables are the tables the v13→v14 step adds content_hash to.
var contentHashTables = []string{"entities", "relations", "marked_entities", "marked_relations"}

// TestMigrateToContentHash pins that a v13 database gains content_hash on the
// live and soft-delete tables, that existing rows read as "not known" (NULL),
// that running the step again is harmless, and that the trigger clears a
// stored hash when the row's content changes (BUG-1DWMYO).
func TestMigrateToContentHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v13.db")
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
	require.NoError(t, err)
	require.NoError(t, db.Close())

	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = raw.Close() }()
	for _, q := range []string{
		`DROP TRIGGER entities_clear_content_hash`, `DROP TRIGGER relations_clear_content_hash`,
	} {
		_, err = raw.Exec(q)
		require.NoError(t, err, q)
	}
	for _, table := range contentHashTables {
		_, err = raw.Exec("ALTER TABLE " + table + " DROP COLUMN content_hash")
		require.NoError(t, err, table)
	}
	_, err = raw.Exec(`INSERT INTO entities (id, type, updated_at)
		VALUES ('FEAT-1', 'feature', '2026-01-01T00:00:00.000000000Z')`)
	require.NoError(t, err)

	// Stamping v13 before each open makes the second open run the step again
	// over columns that now exist.
	for i := range 2 {
		_, err = raw.Exec(`PRAGMA user_version = 13`)
		require.NoError(t, err)
		reopened, openErr := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, openErr, "open %d", i)
		require.NoError(t, reopened.Close())
	}

	for _, table := range contentHashTables {
		var n int
		require.NoError(t, raw.QueryRow(
			`SELECT count(*) FROM pragma_table_info(?) WHERE name = 'content_hash'`, table).Scan(&n))
		require.Equalf(t, 1, n, "%s lacks content_hash after migrating", table)
	}
	readHash := func() *string {
		t.Helper()
		var h *string
		require.NoError(t, raw.QueryRow(`SELECT content_hash FROM entities WHERE id = 'FEAT-1'`).Scan(&h))
		return h
	}
	require.Nil(t, readHash(), "a pre-existing row must read as not known")

	_, err = raw.Exec(`UPDATE entities SET content_hash = 'h1' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	_, err = raw.Exec(`UPDATE entities SET content = '' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	require.NotNil(t, readHash(), "an update that changes nothing must keep the hash")

	_, err = raw.Exec(`UPDATE entities SET content = 'changed' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	require.Nil(t, readHash(), "a content change must clear the stored hash")
}
