package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestMigration0045_ChannelThreads pins migration 0045 (TKT-ETL233O): a
// schema at version 44 gains channel_messages.last_reply_at, in_stream and
// both thread indexes; the backfill keeps top-level messages and replies
// that moved their session in the stream and takes every other reply out,
// including one whose text holds the escaped member; and a second run
// changes nothing.
func TestMigration0045_ChannelThreads(t *testing.T) {
	pool := newScopedPool(t) // migrated to the latest version
	ctx := context.Background()
	for _, stmt := range []string{
		`DROP INDEX channel_messages_thread_idx`,
		`DROP INDEX channel_messages_stream_idx`,
		`ALTER TABLE channel_messages DROP COLUMN last_reply_at`,
		`ALTER TABLE channel_messages DROP COLUMN in_stream`,
		`UPDATE schema_version SET version = 44`,
	} {
		_, err := pool.Exec(ctx, stmt)
		require.NoError(t, err, stmt)
	}
	_, err := pool.Exec(ctx, `INSERT INTO channels (id, anchor_id, type, state, created_by, created_at)
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
		{9, 1, `{"type":"x","props":{"body":"\u0000"},"moved_session":true}`},
	}
	for _, r := range rows {
		_, err = pool.Exec(ctx, `INSERT INTO channel_messages (channel_id, seq, rev, reply_to, doc)
			VALUES (1, $1, $1, $2, $3)`, r.seq, r.replyTo, r.doc)
		require.NoError(t, err)
	}

	require.NoError(t, pgstore.Migrate(ctx, pool))
	require.NoError(t, pgstore.Migrate(ctx, pool), "a second run is harmless")

	want := map[int]int{1: 1, 2: 0, 3: 1, 4: 0, 5: 0, 6: 1, 7: 0, 8: 0, 9: 0}
	for seq, inStream := range want {
		var got int
		var last int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT in_stream, last_reply_at FROM channel_messages WHERE seq = $1`,
			seq).Scan(&got, &last))
		require.Equal(t, inStream, got, "in_stream of seq %d", seq)
		require.Zero(t, last, "last_reply_at of seq %d", seq)
	}
	var indexes int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema()
		AND indexname IN ('channel_messages_thread_idx', 'channel_messages_stream_idx')`).Scan(&indexes))
	require.Equal(t, 2, indexes)
	current, target, err := pgstore.Status(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, target, current)
}
