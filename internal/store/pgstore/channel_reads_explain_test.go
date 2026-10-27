package pgstore_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels/sqlchannels"
)

// TestChannelReadsExplain pins the plans of the read-marker queries (TKT-2B2AIA
// AC 10) on a fixture big enough that a sequential scan would be the
// cheaper guess without the indexes: the unread count reads
// channel_messages_counts_idx, and the read window finds the reader's
// markers through channel_reads_reader_idx. The statements are the ones
// sqlchannels runs ([sqlchannels.ReadSQL]), prepared and explained with
// arguments.
func TestChannelReadsExplain(t *testing.T) {
	pool := newScopedPool(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`INSERT INTO channels (id, anchor_id, type, state, created_by, created_at, last_seq, last_at)
			SELECT g, 'TKT-' || g, 'room', 'open', '{}', 1, 100, g FROM generate_series(1, 300) g`,
		`INSERT INTO channel_messages (channel_id, seq, rev, doc, counts)
			SELECT c, s, s, '{}', CASE WHEN s % 3 = 0 THEN 0 ELSE 1 END
			FROM generate_series(1, 300) c, generate_series(1, 100) s`,
		`INSERT INTO channel_reads (channel_id, reader_user, reader_org, seq, at)
			SELECT c, 'user-' || u, '', 50, 1 FROM generate_series(1, 300) c, generate_series(1, 30) u`,
		`ANALYZE channels`,
		`ANALYZE channel_messages`,
		`ANALYZE channel_reads`,
	} {
		_, err := pool.Exec(ctx, stmt)
		require.NoError(t, err, stmt)
	}
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	sql := sqlchannels.ReadSQL(2)

	mark := explainPrepared(t, conn, "mark_state", sql.Mark, `7, 'user-7', ''`)
	require.Contains(t, mark, "channel_messages_counts_idx")

	reads := explainPrepared(t, conn, "read_chunk", sql.Reads,
		`'user-7', '', 'TKT-1', '', 'room', '', 'TKT-2', '', 'room', ''`)
	require.Contains(t, reads, "channel_messages_counts_idx")

	window := explainPrepared(t, conn, "read_window", sql.Window, `'user-7', '', 500`)
	require.Contains(t, window, "channel_reads_reader_idx")
}

// explainPrepared prepares query, with its `?` placeholders numbered, as
// name on conn and returns the EXPLAIN output of running it with args.
func explainPrepared(t *testing.T, conn *pgxpool.Conn, name, query, args string) string {
	t.Helper()
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteRune(r)
	}
	_, err := conn.Exec(context.Background(), "PREPARE "+name+" AS "+b.String())
	require.NoError(t, err, name)
	rows, err := conn.Query(context.Background(), "EXPLAIN EXECUTE "+name+"("+args+")")
	require.NoError(t, err, name)
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		lines = append(lines, line)
	}
	require.NoError(t, rows.Err())
	plan := strings.Join(lines, "\n")
	t.Logf("%s plan:\n%s", name, plan)
	return plan
}
