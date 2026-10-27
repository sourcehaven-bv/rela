package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// creatorIndexSQL returns the stored definition of channels_creator_idx, or
// "" when the index is absent.
func creatorIndexSQL(t *testing.T, raw *sql.DB) string {
	t.Helper()
	var def sql.NullString
	err := raw.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'channels_creator_idx'`).
		Scan(&def)
	if err == sql.ErrNoRows {
		return ""
	}
	require.NoError(t, err)
	return def.String
}

// TestMigrate_v26ToV27ChannelCreator pins the v26 to v27 rung (TKT-ZZ4PNX
// AC 4): a version-26 file gains created_by_user and the creator index with
// the shape of a fresh database; a row with a valid author gets its user; a
// row whose created_by holds a NUL escape, is not JSON, or has a user that
// is not a string keeps the empty string; and opening a second time is harmless.
func TestMigrate_v26ToV27ChannelCreator(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	wantShape, wantIndex := channelsShape(t, fresh), creatorIndexSQL(t, fresh)
	require.NotEmpty(t, wantIndex)

	path := filepath.Join(t.TempDir(), "v26.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`DROP INDEX channels_creator_idx`,
		`ALTER TABLE channels DROP COLUMN created_by_user`,
		// last_at comes after created_by_user; the v28 rung adds it back.
		`ALTER TABLE channels DROP COLUMN last_at`,
		`PRAGMA user_version = 26`,
	} {
		_, err := raw.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	rows := map[string]string{
		"TKT-1": `{"user":"alice","tool":"cli"}`,
		"TKT-2": `{"user":"bob\u0000x"}`,
		"TKT-3": `not json`,
		"TKT-4": `{"user":7}`,
		"TKT-5": `{"tool":"cli"}`,
	}
	for id, by := range rows {
		_, err := raw.Exec(`INSERT INTO channels (anchor_id, type, state, created_by, created_at)
			VALUES (?, 'room', 'open', ?, 1)`, id, by)
		require.NoError(t, err)
	}

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, wantShape, channelsShape(t, raw), "an upgraded table must match a fresh one")
	require.Equal(t, wantIndex, creatorIndexSQL(t, raw))
	want := map[string]string{"TKT-1": "alice", "TKT-2": "", "TKT-3": "", "TKT-4": "", "TKT-5": ""}
	for id, user := range want {
		var got string
		require.NoError(t, raw.QueryRow(`SELECT created_by_user FROM channels WHERE anchor_id = ?`, id).Scan(&got))
		require.Equal(t, user, got, id)
	}
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
