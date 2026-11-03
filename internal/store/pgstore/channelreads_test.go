package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestMigration0035_ChannelReads pins migration 0035 (TKT-2B2AIA AC 6): a
// schema at version 34 gains channel_reads, channel_messages.counts,
// channels.last_at and both indexes; the backfill sets counts to 0 on
// removed messages and session notices and keeps 1 on every other row,
// including a doc that is not JSON or holds the \u0000 escape jsonb
// refuses; last_at is the latest lifecycle time; a second run changes
// nothing; and the migration leaves no helper function.
func TestMigration0035_ChannelReads(t *testing.T) {
	pool := newScopedPool(t) // migrated to the latest version
	ctx := context.Background()
	for _, stmt := range []string{
		`DROP INDEX channel_messages_counts_idx`,
		`DROP INDEX channel_reads_reader_idx`,
		`DROP TABLE channel_reads`,
		`ALTER TABLE channel_messages DROP COLUMN counts`,
		`ALTER TABLE channels DROP COLUMN last_at`,
		`UPDATE schema_version SET version = 34`,
	} {
		_, err := pool.Exec(ctx, stmt)
		require.NoError(t, err, stmt)
	}
	_, err := pool.Exec(ctx, `INSERT INTO channels (id, anchor_id, type, state, created_by, created_at, closed_at,
		archived_at, reopened_at) VALUES (1, 'TKT-1', 'room', 'open', '{}', 10, 40, 30, 20),
		(2, 'TKT-2', 'room', 'open', '{}', 50, 0, 0, 0)`)
	require.NoError(t, err)
	docs := map[int]string{
		1: `{"type":"text","props":{"body":"hi"}}`,
		2: `{"removed":{"by":{"user":"alice"}}}`,
		3: `{"type":"session","triggered_by":"session:room"}`,
		4: `{"type":"reminder","triggered_by":"reminder:room/x"}`,
		5: `not json "removed":`,
		6: `{"removed":{"by":{"user":"a\u0000b"}}}`,
		7: `{"type":"text","props":{"removed":true}}`,
	}
	for seq, doc := range docs {
		_, err = pool.Exec(ctx, `INSERT INTO channel_messages (channel_id, seq, rev, doc) VALUES (1, $1, $1, $2)`,
			seq, doc)
		require.NoError(t, err)
	}

	require.NoError(t, pgstore.Migrate(ctx, pool))
	require.NoError(t, pgstore.Migrate(ctx, pool), "a second run is harmless")

	want := map[int]int{1: 1, 2: 0, 3: 0, 4: 1, 5: 1, 6: 1, 7: 1}
	for seq, counts := range want {
		var got int
		require.NoError(t, pool.QueryRow(ctx, `SELECT counts FROM channel_messages WHERE seq = $1`, seq).Scan(&got))
		require.Equal(t, counts, got, "seq %d", seq)
	}
	for id, at := range map[int]int64{1: 40, 2: 50} {
		var got int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT last_at FROM channels WHERE id = $1`, id).Scan(&got))
		require.Equal(t, at, got, "channel %d", id)
	}
	var indexes, functions int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema()
		AND indexname IN ('channel_messages_counts_idx', 'channel_reads_reader_idx')`).Scan(&indexes))
	require.Equal(t, 2, indexes)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_proc WHERE proname = 'rela_message_counts'`).
		Scan(&functions))
	require.Zero(t, functions)
	current, target, err := pgstore.Status(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, target, current)
}
