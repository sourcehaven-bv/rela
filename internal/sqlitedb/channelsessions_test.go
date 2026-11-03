package sqlitedb_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// channelsShape renders the channels table's columns like
// channelTypesShape.
func channelsShape(t *testing.T, raw *sql.DB) []string {
	t.Helper()
	rows, err := raw.Query(`SELECT name, type, "notnull", coalesce(dflt_value, 'NULL'), pk
		FROM pragma_table_info('channels') ORDER BY cid`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var name, typ, dflt string
		var notNull, pk int
		require.NoError(t, rows.Scan(&name, &typ, &notNull, &dflt, &pk))
		out = append(out, fmt.Sprintf("%s %s notnull=%d default=%s pk=%d", name, typ, notNull, dflt, pk))
	}
	require.NoError(t, rows.Err())
	return out
}

// TestMigrateChannelSessions pins the v23 to v24 rung: a v23 channels table
// gains the three session columns with the shape a fresh database has, an
// existing row reads as an unset session, and running the rung again is
// harmless.
func TestMigrateChannelSessions(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	want := channelsShape(t, fresh)

	path := filepath.Join(t.TempDir(), "v23.db")
	raw := openChannelDB(t, path)
	// A v23 table has neither the session, the v25 timer, the v27
	// creator nor the v28 last-activity columns.
	for _, idx := range []string{"channels_session_due_idx", "channels_creator_idx"} {
		_, err := raw.Exec(`DROP INDEX ` + idx)
		require.NoError(t, err)
	}
	var err error
	for _, col := range []string{
		"session_state", "session_gen", "session_entered_at", "session_next_due_at", "session_restarts",
		"session_reminders", "created_by_user", "last_at",
	} {
		_, err = raw.Exec(`ALTER TABLE channels DROP COLUMN ` + col)
		require.NoError(t, err)
	}
	_, err = raw.Exec(`INSERT INTO channels (anchor_id, type, state, created_by, created_at)
		VALUES ('TKT-1', 'room', 'open', '{"user":"alice"}', 1)`)
	require.NoError(t, err)

	for i := range 2 {
		_, err = raw.Exec(`PRAGMA user_version = 23`)
		require.NoError(t, err)
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, want, channelsShape(t, raw), "an upgraded table must match a fresh one")
	var state string
	var gen, entered int64
	require.NoError(t, raw.QueryRow(`SELECT session_state, session_gen, session_entered_at FROM channels
		WHERE anchor_id = 'TKT-1'`).Scan(&state, &gen, &entered))
	require.Empty(t, state)
	require.Zero(t, gen)
	require.Zero(t, entered)
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
