package sqlitechannels_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels/sqlchannels"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestAddressed_UsesIndexes pins the plans of the addressed-row statements
// (TKT-6RXAC0 AC 13) that sqlchannels runs ([sqlchannels.AddressedSQL]), on
// a fresh database with no ANALYZE: the inner scan of ListAddressed reads
// channel_mentions_target_idx in index order, with no sort below its LIMIT,
// the scan of one kind reads channel_mentions_kind_idx the same way, and
// the cap statement reads channel_mentions_cap_idx.
func TestAddressed_UsesIndexes(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	sql := sqlchannels.AddressedSQL()

	plan := explain(t, db, sql.List, "alice", "alice", "", 500)
	require.Contains(t, plan, "channel_mentions_target_idx")
	require.NotContains(t, plan, "SCAN channel_mentions\n", "the scan must not read every row")
	// The outer ORDER BY sorts the gated rows (at most 2000); the inner
	// one, the co-routine before "SCAN a", must come from the index.
	inner, _, found := strings.Cut(plan, "SCAN a\n")
	require.True(t, found, "the inner scan runs as a co-routine")
	require.NotContains(t, inner, "TEMP B-TREE", "the inner ORDER BY must not sort")

	plan = explain(t, db, sql.ListKind, "alice", "reply", "alice", "", 500)
	require.Contains(t, plan, "channel_mentions_kind_idx", "a kind scan reads the kind index")
	inner, _, found = strings.Cut(plan, "SCAN a\n")
	require.True(t, found, "the inner scan runs as a co-routine")
	require.NotContains(t, inner, "TEMP B-TREE", "the inner ORDER BY of a kind scan must not sort")

	plan = explain(t, db, sql.Cap, int64(1), "alice")
	require.Contains(t, plan, "channel_mentions_cap_idx")
	require.NotContains(t, plan, "TEMP B-TREE", "the cap rows come in index order")
}
