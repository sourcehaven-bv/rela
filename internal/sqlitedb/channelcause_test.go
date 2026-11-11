package sqlitedb_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestMigrate_v31ToV32SessionCause pins the v31 to v32 rung (TKT-V05706X
// AC 1): a version-31 file gains channels.session_cause with the shape of a
// fresh database, an existing row reads 0, and opening a second time
// changes nothing.
func TestMigrate_v31ToV32SessionCause(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	want := channelsShape(t, fresh)

	path := filepath.Join(t.TempDir(), "v31.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`ALTER TABLE channels DROP COLUMN session_asked`,
		`ALTER TABLE channels DROP COLUMN session_previous`,
		`ALTER TABLE channels DROP COLUMN session_cause`,
		`PRAGMA user_version = 31`,
	} {
		_, err := raw.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	_, err := raw.Exec(`INSERT INTO channels (anchor_id, type, state, created_by, created_at, session_state,
		session_gen) VALUES ('TKT-1', 'room', 'open', '{"user":"alice"}', 1, 'asking', 3)`)
	require.NoError(t, err)

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, want, channelsShape(t, raw), "an upgraded table must match a fresh one")
	var cause int64
	require.NoError(t, raw.QueryRow(`SELECT session_cause FROM channels WHERE anchor_id = 'TKT-1'`).Scan(&cause))
	require.Zero(t, cause, "no backfill")
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
