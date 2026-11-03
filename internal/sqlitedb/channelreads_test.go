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

// tableShape returns the column shape of table, as channelsShape does for
// channels.
func tableShape(t *testing.T, raw *sql.DB, table string) []string {
	t.Helper()
	rows, err := raw.Query(`SELECT name, type, "notnull", coalesce(dflt_value, 'NULL'), pk
		FROM pragma_table_info(?) ORDER BY cid`, table)
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

// indexSQL returns the stored definition of the index name, or "".
func indexSQL(t *testing.T, raw *sql.DB, name string) string {
	t.Helper()
	var def sql.NullString
	err := raw.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?`, name).Scan(&def)
	if err == sql.ErrNoRows {
		return ""
	}
	require.NoError(t, err)
	return def.String
}

// TestMigrate_v27ToV28ChannelReads pins the v27 to v28 rung (TKT-2B2AIA
// AC 6): a version-27 file gains channel_reads, channel_messages.counts,
// channels.last_at and both indexes with the shape of a fresh database;
// the backfill sets counts to 0 on removed messages and session notices
// and keeps 1 on every other row, including a doc that is not JSON or
// holds a NUL escape; last_at is the latest lifecycle time; and opening a
// second time changes nothing.
func TestMigrate_v27ToV28ChannelReads(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	wantChannels := channelsShape(t, fresh)
	wantMessages := tableShape(t, fresh, "channel_messages")
	wantReads := tableShape(t, fresh, "channel_reads")
	wantCounts := indexSQL(t, fresh, "channel_messages_counts_idx")
	wantReader := indexSQL(t, fresh, "channel_reads_reader_idx")
	require.NotEmpty(t, wantReads)
	require.NotEmpty(t, wantCounts)
	require.NotEmpty(t, wantReader)

	path := filepath.Join(t.TempDir(), "v27.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`DROP INDEX channel_messages_counts_idx`,
		`DROP INDEX channel_reads_reader_idx`,
		`DROP TABLE channel_reads`,
		`ALTER TABLE channel_messages DROP COLUMN counts`,
		`ALTER TABLE channels DROP COLUMN last_at`,
		`PRAGMA user_version = 27`,
	} {
		_, err := raw.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	_, err := raw.Exec(`INSERT INTO channels (id, anchor_id, type, state, created_by, created_at, closed_at,
		archived_at, reopened_at) VALUES (1, 'TKT-1', 'room', 'open', '{}', 10, 40, 30, 20),
		(2, 'TKT-2', 'room', 'open', '{}', 50, 0, 0, 0)`)
	require.NoError(t, err)
	docs := map[int]string{
		1: `{"type":"text","props":{"body":"hi"}}`,
		2: `{"removed":{"by":{"user":"alice"}}}`,
		3: `{"type":"session","triggered_by":"session:room"}`,
		4: `{"type":"reminder","triggered_by":"reminder:room/x"}`,
		5: `not json`,
		6: `{"removed":{"by":{"user":"a\u0000b"}}}`,
	}
	for seq, doc := range docs {
		_, err := raw.Exec(`INSERT INTO channel_messages (channel_id, seq, rev, doc) VALUES (1, ?, ?, ?)`,
			seq, seq, doc)
		require.NoError(t, err)
	}

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, wantChannels, channelsShape(t, raw), "an upgraded channels table must match a fresh one")
	require.Equal(t, wantMessages, tableShape(t, raw, "channel_messages"))
	require.Equal(t, wantReads, tableShape(t, raw, "channel_reads"))
	require.Equal(t, wantCounts, indexSQL(t, raw, "channel_messages_counts_idx"))
	require.Equal(t, wantReader, indexSQL(t, raw, "channel_reads_reader_idx"))
	want := map[int]int{1: 1, 2: 0, 3: 0, 4: 1, 5: 1, 6: 1}
	for seq, counts := range want {
		var got int
		require.NoError(t, raw.QueryRow(`SELECT counts FROM channel_messages WHERE seq = ?`, seq).Scan(&got))
		require.Equal(t, counts, got, "seq %d", seq)
	}
	for id, at := range map[int]int64{1: 40, 2: 50} {
		var got int64
		require.NoError(t, raw.QueryRow(`SELECT last_at FROM channels WHERE id = ?`, id).Scan(&got))
		require.Equal(t, at, got, "channel %d", id)
	}
	var version int
	require.NoError(t, raw.QueryRow(`PRAGMA user_version`).Scan(&version))
	require.Equal(t, sqlitedb.SchemaVersion(), version)
}
