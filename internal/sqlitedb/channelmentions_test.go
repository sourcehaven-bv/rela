package sqlitedb_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestMigrate_v29ToV30ChannelMentions pins the v29 to v30 rung (TKT-6RXAC0
// AC 9): a version-29 file gains channel_mentions and its three indexes with
// the shape of a fresh database, old messages gain no rows, and opening a
// second time changes nothing.
func TestMigrate_v29ToV30ChannelMentions(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	wantTable := tableShape(t, fresh, "channel_mentions")
	wantTarget := indexSQL(t, fresh, "channel_mentions_target_idx")
	wantCap := indexSQL(t, fresh, "channel_mentions_cap_idx")
	wantKind := indexSQL(t, fresh, "channel_mentions_kind_idx")
	require.NotEmpty(t, wantTable)
	require.NotEmpty(t, wantTarget)
	require.NotEmpty(t, wantCap)
	require.NotEmpty(t, wantKind)

	path := filepath.Join(t.TempDir(), "v29.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`DROP INDEX channel_mentions_target_idx`,
		`DROP INDEX channel_mentions_cap_idx`,
		`DROP INDEX channel_mentions_kind_idx`,
		`DROP TABLE channel_mentions`,
		`PRAGMA user_version = 29`,
	} {
		_, err := raw.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	_, err := raw.Exec(`INSERT INTO channels (id, anchor_id, type, state, created_by, created_at, closed_at,
		archived_at, reopened_at) VALUES (1, 'TKT-1', 'room', 'open', '{}', 10, 0, 0, 0)`)
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO channel_messages (channel_id, seq, rev, doc)
		VALUES (1, 1, 1, '{"type":"text","props":{"body":"` + "`USR-7`" + `"}}')`)
	require.NoError(t, err)

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, wantTable, tableShape(t, raw, "channel_mentions"))
	require.Equal(t, wantTarget, indexSQL(t, raw, "channel_mentions_target_idx"))
	require.Equal(t, wantCap, indexSQL(t, raw, "channel_mentions_cap_idx"))
	require.Equal(t, wantKind, indexSQL(t, raw, "channel_mentions_kind_idx"))
	var rows int
	require.NoError(t, raw.QueryRow(`SELECT count(*) FROM channel_mentions`).Scan(&rows))
	require.Zero(t, rows, "no backfill")
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, 30, version)
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
