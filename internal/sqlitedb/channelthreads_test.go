package sqlitedb_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestMigrate_v36ToV37ChannelThreads pins the v36 to v37 rung
// (TKT-ETL233O): a version-36 file gains channel_messages.last_reply_at,
// in_stream and both thread indexes with the shape of a fresh database;
// the backfill keeps top-level messages and replies that moved their
// session in the stream and takes every other reply out; and opening a
// second time changes nothing.
func TestMigrate_v36ToV37ChannelThreads(t *testing.T) {
	fresh := openChannelDB(t, filepath.Join(t.TempDir(), "fresh.db"))
	wantMessages := tableShape(t, fresh, "channel_messages")
	wantThread := indexSQL(t, fresh, "channel_messages_thread_idx")
	wantStream := indexSQL(t, fresh, "channel_messages_stream_idx")
	require.NotEmpty(t, wantThread)
	require.NotEmpty(t, wantStream)

	path := filepath.Join(t.TempDir(), "v36.db")
	raw := openChannelDB(t, path)
	for _, stmt := range []string{
		`DROP INDEX channel_messages_thread_idx`,
		`DROP INDEX channel_messages_stream_idx`,
		`ALTER TABLE channel_messages DROP COLUMN last_reply_at`,
		`ALTER TABLE channel_messages DROP COLUMN in_stream`,
		`PRAGMA user_version = 36`,
	} {
		_, err := raw.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	_, err := raw.Exec(`INSERT INTO channels (id, anchor_id, type, state, created_by, created_at)
		VALUES (1, 'TKT-1', 'room', 'open', '{}', 10)`)
	require.NoError(t, err)
	rows := []struct {
		seq, replyTo int
		doc          string
	}{
		{1, 0, `{"type":"ask","props":{"body":"q"}}`},
		{2, 1, `{"type":"text","props":{"body":"r"}}`},
		{3, 1, `{"type":"answer","moved_session":true}`},
		{4, 1, `{"type":"text","props":{"body":"\"moved_session\":true"}}`},
		{5, 1, `{"removed":{"by":{"user":"alice"}}}`},
		{6, 0, `not json`},
		{7, 1, `{"type":"text","props":{"moved_session":true}}`},
		{8, 1, `not json "moved_session":true`},
	}
	for _, r := range rows {
		_, err := raw.Exec(`INSERT INTO channel_messages (channel_id, seq, rev, reply_to, doc) VALUES (1, ?, ?, ?, ?)`,
			r.seq, r.seq, r.replyTo, r.doc)
		require.NoError(t, err)
	}

	for i := range 2 {
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: path})
		require.NoErrorf(t, err, "open %d", i)
		require.NoError(t, db.Close())
	}

	require.Equal(t, wantMessages, tableShape(t, raw, "channel_messages"))
	require.Equal(t, wantThread, indexSQL(t, raw, "channel_messages_thread_idx"))
	require.Equal(t, wantStream, indexSQL(t, raw, "channel_messages_stream_idx"))
	want := map[int]int{1: 1, 2: 0, 3: 1, 4: 0, 5: 0, 6: 1, 7: 0, 8: 0}
	for seq, inStream := range want {
		var got, last int
		require.NoError(t, raw.QueryRow(`SELECT in_stream, last_reply_at FROM channel_messages WHERE seq = ?`, seq).
			Scan(&got, &last))
		require.Equalf(t, inStream, got, "in_stream of seq %d", seq)
		require.Zerof(t, last, "last_reply_at of seq %d", seq)
	}
}
