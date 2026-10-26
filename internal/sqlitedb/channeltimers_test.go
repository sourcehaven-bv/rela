package sqlitedb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// v24ChannelsDDL is the channels table as schema version 24 created it,
// kept as a fixture: TestOpenV24Database checks that the file it opens has
// exactly this shape before the upgrade.
const v24ChannelsDDL = `
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
	UNIQUE (anchor_id, anchor_face, type, name, tomb)
) STRICT;`

// dueIndexSQL reads the definition of the session due index, or "" when
// the database has none.
func dueIndexSQL(t *testing.T, raw *sql.DB) string {
	t.Helper()
	var def sql.NullString
	err := raw.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'channels_session_due_idx'`).
		Scan(&def)
	if err == sql.ErrNoRows {
		return ""
	}
	require.NoError(t, err)
	return def.String
}

// TestOpenV24Database pins the v24 to v25 rung (TKT-INM3AZ AC 7): a real
// version-24 database file opens through Open, gains the timer columns and
// the due index with the shape of a fresh database, an existing row reads
// with no due time and no restart anchors, and opening a second time is
// harmless. The due index runs after the ladder; were it in schemaSQL, Open
// would fail on this file before the rung adds the column.
func TestOpenV24Database(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	wantShape, wantIndex := channelsShape(t, fresh), dueIndexSQL(t, fresh)
	require.NotEmpty(t, wantIndex)

	fixture, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "fixture.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = fixture.Close() })
	_, err = fixture.Exec(v24ChannelsDDL)
	require.NoError(t, err)
	v24Shape := channelsShape(t, fixture)

	path := filepath.Join(t.TempDir(), "v24.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`DROP INDEX channels_session_due_idx`,
		`ALTER TABLE channels DROP COLUMN session_next_due_at`,
		`ALTER TABLE channels DROP COLUMN session_restarts`,
		`ALTER TABLE channels DROP COLUMN session_reminders`,
		`PRAGMA user_version = 24`,
	} {
		_, err = raw.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	require.Equal(t, v24Shape, channelsShape(t, raw), "the file must hold the v24 channels table")
	require.Empty(t, dueIndexSQL(t, raw))
	_, err = raw.Exec(`INSERT INTO channels (anchor_id, type, state, created_by, created_at, session_state, session_gen)
		VALUES ('TKT-1', 'room', 'open', '{"user":"alice"}', 1, 'detected', 1)`)
	require.NoError(t, err)

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, wantShape, channelsShape(t, raw), "an upgraded table must match a fresh one")
	require.Equal(t, wantIndex, dueIndexSQL(t, raw))
	var due int64
	var restarts string
	require.NoError(t, raw.QueryRow(`SELECT session_next_due_at, session_restarts FROM channels
		WHERE anchor_id = 'TKT-1'`).Scan(&due, &restarts))
	require.Zero(t, due)
	require.Empty(t, restarts)
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
