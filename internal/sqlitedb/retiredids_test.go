package sqlitedb_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestMigrate_v30ToV31RetiredIDs pins the v30 to v31 rung (TKT-J6GISC3): a
// version-30 file gains retired_ids and its indexes with the shape of a
// fresh database, and opening a second time changes nothing.
func TestMigrate_v30ToV31RetiredIDs(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	wantTable := tableShape(t, fresh, "retired_ids")
	wantLower := indexSQL(t, fresh, "retired_ids_id_lower_key")
	wantSuccessor := indexSQL(t, fresh, "retired_ids_successor_idx")
	require.NotEmpty(t, wantTable)
	require.NotEmpty(t, wantLower)
	require.NotEmpty(t, wantSuccessor)

	path := filepath.Join(t.TempDir(), "v30.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`DROP TABLE retired_ids`,
		`PRAGMA user_version = 30`,
	} {
		_, err := raw.Exec(stmt)
		require.NoError(t, err, stmt)
	}

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, wantTable, tableShape(t, raw, "retired_ids"))
	require.Equal(t, wantLower, indexSQL(t, raw, "retired_ids_id_lower_key"))
	require.Equal(t, wantSuccessor, indexSQL(t, raw, "retired_ids_successor_idx"))
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
