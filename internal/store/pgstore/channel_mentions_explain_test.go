package pgstore_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels/sqlchannels"
)

// TestChannelMentionsExplain pins the plans of the addressed-row
// statements (TKT-6RXAC0 AC 13) on a fixture big enough that a sequential
// scan would be the cheaper guess without the indexes: the inner scan of
// ListAddressed is an Index Scan of channel_mentions_target_idx under its
// Limit, with no Sort node below it, the scan of one kind is the same over
// channel_mentions_kind_idx, and the cap statement reads
// channel_mentions_cap_idx. The statements are the ones sqlchannels runs
// ([sqlchannels.AddressedSQL]).
func TestChannelMentionsExplain(t *testing.T) {
	pool := newScopedPool(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`INSERT INTO channels (id, anchor_id, type, state, created_by, created_at, last_seq, last_at)
			SELECT g, 'TKT-' || g, 'room', 'open', '{}', 1, 100, g FROM generate_series(1, 300) g`,
		`INSERT INTO channel_mentions (channel_id, seq, target, kind, at, author_user, msg_type)
			SELECT c, s, 'user-' || (s % 3), CASE WHEN s % 2 = 0 THEN 'reply' ELSE 'mention' END,
				c * 1000 + s, 'alice', 'text'
			FROM generate_series(1, 300) c, generate_series(1, 100) s`,
		`ANALYZE channels`,
		`ANALYZE channel_mentions`,
	} {
		_, err := pool.Exec(ctx, stmt)
		require.NoError(t, err, stmt)
	}
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	sql := sqlchannels.AddressedSQL()

	// user-1 holds 10000 rows, so reading 2000 in index order beats
	// sorting them all. Half its rows are replies, 5000 of them, so a reply
	// scan also cuts at 2000 in index order, and through the target index
	// it would read and drop a mention for each reply it keeps.
	list := explainPrepared(t, conn, "addressed_list", sql.List, `'user-1', 'user-1', '', 500`)
	require.Contains(t, list, "channel_mentions_target_idx")
	require.Equal(t, "Limit", indexScanParent(t, list, "channel_mentions_target_idx"),
		"the inner Limit reads the Index Scan directly, with no Sort between")

	kind := explainPrepared(t, conn, "addressed_list_kind", sql.ListKind, `'user-1', 'reply', 'user-1', '', 500`)
	require.Contains(t, kind, "channel_mentions_kind_idx")
	require.Equal(t, "Limit", indexScanParent(t, kind, "channel_mentions_kind_idx"),
		"the inner Limit of a kind scan reads the Index Scan directly, with no Sort between")

	capPlan := explainPrepared(t, conn, "addressed_cap", sql.Cap, `7, 'user-1'`)
	require.Contains(t, capPlan, "channel_mentions_cap_idx")
}

// indexScanParent returns the node right above the Index Scan of index in
// plan, by indentation: "Limit" when the scan feeds a Limit directly.
func indexScanParent(t *testing.T, plan, index string) string {
	t.Helper()
	lines := strings.Split(plan, "\n")
	indent := func(l string) int { return len(l) - len(strings.TrimLeft(l, " ->")) }
	for i, l := range lines {
		if !strings.Contains(l, "Index Scan") || !strings.Contains(l, index) {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			isNode := j == 0 || strings.HasPrefix(strings.TrimLeft(lines[j], " "), "->")
			if isNode && indent(lines[j]) < indent(l) {
				node := strings.TrimLeft(lines[j], " ->")
				name, _, _ := strings.Cut(node, "  (")
				return name
			}
		}
	}
	t.Fatalf("no Index Scan of %s in the plan", index)
	return ""
}
