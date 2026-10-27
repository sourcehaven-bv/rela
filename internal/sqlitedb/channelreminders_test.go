package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// v25ChannelsDDL is the channels table as schema version 25 created it,
// kept as a fixture together with its due index.
const v25ChannelsDDL = `
CREATE TABLE channels (
	id          INTEGER PRIMARY KEY,
	anchor_id   TEXT NOT NULL,
	anchor_face TEXT NOT NULL DEFAULT '',
	type        TEXT NOT NULL,
	name        TEXT NOT NULL DEFAULT '',
	tomb        TEXT NOT NULL DEFAULT '',
	incarnation TEXT NOT NULL DEFAULT '',
	state       TEXT NOT NULL,
	title       TEXT NOT NULL DEFAULT '',
	created_by  TEXT NOT NULL,
	created_at  INTEGER NOT NULL,
	closed_at   INTEGER NOT NULL DEFAULT 0,
	archived_at INTEGER NOT NULL DEFAULT 0,
	reopened_at INTEGER NOT NULL DEFAULT 0,
	last_seq    INTEGER NOT NULL DEFAULT 0,
	rev         INTEGER NOT NULL DEFAULT 0,
	session_state      TEXT NOT NULL DEFAULT '',
	session_gen        INTEGER NOT NULL DEFAULT 0,
	session_entered_at INTEGER NOT NULL DEFAULT 0,
	session_next_due_at INTEGER NOT NULL DEFAULT 0,
	session_restarts    TEXT NOT NULL DEFAULT '',
	UNIQUE (anchor_id, anchor_face, type, name, tomb)
) STRICT;`

// TestOpenV25Database pins the v25 to v26 rung (TKT-36Z6X3 AC 8): a real
// version-25 database file, whose channels table is the v25 fixture, opens
// through Open and gains session_reminders with the shape of a fresh
// database; an existing row reads with no reminder records; opening a
// second time is harmless; and user_version is the current version.
func TestOpenV25Database(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	wantShape := channelsShape(t, fresh)

	fixture, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "fixture.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = fixture.Close() })
	_, err = fixture.Exec(v25ChannelsDDL)
	require.NoError(t, err)
	v25Shape := channelsShape(t, fixture)

	path := filepath.Join(t.TempDir(), "v25.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`DROP INDEX channels_creator_idx`,
		`ALTER TABLE channels DROP COLUMN session_reminders`,
		`ALTER TABLE channels DROP COLUMN created_by_user`,
		`PRAGMA user_version = 25`,
	} {
		_, err = raw.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	require.Equal(t, v25Shape, channelsShape(t, raw), "the file must hold the v25 channels table")
	_, err = raw.Exec(`INSERT INTO channels (anchor_id, type, state, created_by, created_at, session_state,
		session_gen, session_next_due_at) VALUES ('TKT-1', 'room', 'open', '{"user":"alice"}', 1, 'detected', 1, 5)`)
	require.NoError(t, err)

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, wantShape, channelsShape(t, raw), "an upgraded table must match a fresh one")
	var reminders string
	var due int64
	require.NoError(t, raw.QueryRow(`SELECT session_reminders, session_next_due_at FROM channels
		WHERE anchor_id = 'TKT-1'`).Scan(&reminders, &due))
	require.Empty(t, reminders)
	require.EqualValues(t, 5, due)
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
