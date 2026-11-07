package sqlitedb_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestMigrate_v32ToV33SessionPrevious pins the v32 to v33 rung
// (TKT-56TPOTE): a version-32 file gains channels.session_previous with the
// shape of a fresh database, an existing row reads empty, and opening a
// second time changes nothing.
func TestMigrate_v32ToV33SessionPrevious(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	want := channelsShape(t, fresh)

	path := filepath.Join(t.TempDir(), "v32.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`ALTER TABLE channels DROP COLUMN session_previous`,
		`PRAGMA user_version = 32`,
	} {
		_, err := raw.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	_, err := raw.Exec(`INSERT INTO channels (anchor_id, type, state, created_by, created_at, session_state,
		session_gen, session_cause) VALUES ('TKT-1', 'room', 'open', '{"user":"alice"}', 1, 'asking', 3, 7)`)
	require.NoError(t, err)

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, want, channelsShape(t, raw), "an upgraded table must match a fresh one")
	var previous string
	require.NoError(t, raw.QueryRow(`SELECT session_previous FROM channels WHERE anchor_id = 'TKT-1'`).Scan(&previous))
	require.Empty(t, previous, "no backfill")
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
