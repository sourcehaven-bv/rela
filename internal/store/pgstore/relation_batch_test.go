//go:build postgres

package pgstore_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// seedBatchEdges creates 600 sources P-i with two "contains" edges each, and
// returns the 600 source ids and the 1,200 target ids.
func seedBatchEdges(t *testing.T, st store.Store) (from, to []string) {
	t.Helper()
	ctx := context.Background()
	for i := range 600 {
		src := fmt.Sprintf("P-%04d", i)
		from = append(from, src)
		for _, side := range []string{"a", "b"} {
			key := entity.RelationKey{From: src, Type: "contains", To: fmt.Sprintf("C-%04d-%s", i, side)}
			_, err := st.CreateRelation(ctx, key, &store.RelationData{})
			require.NoError(t, err)
			to = append(to, key.To)
		}
	}
	return from, to
}

// listBatch lists the outgoing "contains" edges of q and returns the distinct
// edges it saw.
func listBatch(ctx context.Context, t *testing.T, st store.Store, q store.RelationQuery) map[string]bool {
	t.Helper()
	q.Type, q.Direction = "contains", store.DirectionOutgoing
	seen := map[string]bool{}
	for rel, err := range st.ListRelations(ctx, q) {
		require.NoError(t, err)
		seen[rel.From+"->"+rel.To] = true
	}
	return seen
}

// A batch over the one-shot threshold reads pages as large as the ids it
// carries (pagePlan, TKT-K7EM7Z5).
func TestListRelations_LargeIDBatchPages(t *testing.T) {
	st := openWriter(t, freshFeedSchema(t))
	from, to := seedBatchEdges(t, st)

	t.Run("ids sent once", func(t *testing.T) {
		// At the fixed page these 1,200 rows took three statements that each
		// carried all 1,800 ids.
		ctx, stats := store.WithQueryStats(context.Background())
		require.Len(t, listBatch(ctx, t, st, store.RelationQuery{EntityIDs: from, PeerIDs: to}), 1200)
		require.EqualValues(t, 1, stats.Queries())
	})
	t.Run("pages past the batch", func(t *testing.T) {
		// 600 ids match 1,200 rows: pages of 600, 600 and an empty one. The
		// later pages carry keyset arguments after the exec mode.
		ctx, stats := store.WithQueryStats(context.Background())
		require.Len(t, listBatch(ctx, t, st, store.RelationQuery{EntityIDs: from}), 1200)
		require.EqualValues(t, 3, stats.Queries())
	})
}

// A batch over the one-shot threshold runs as an unnamed statement, so
// PostgreSQL never reuses a generic plan for it (pagePlan).
//
// The pool has one connection, so pg_prepared_statements counts what the
// store's statements left behind on it. The small batch is the control: it
// proves a cached statement would show up there.
func TestListRelations_LargeIDBatchIsNotPrepared(t *testing.T) {
	dsn := dsnForSchema(t, freshFeedSchema(t))
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	st, _, err := pgstore.Open(ctx, pool, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	from, _ := seedBatchEdges(t, st)
	prepared := func() int {
		t.Helper()
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_prepared_statements`).Scan(&n))
		return n
	}

	prepared() // the count statement itself is cached on its first run
	before := prepared()
	require.Len(t, listBatch(ctx, t, st, store.RelationQuery{EntityIDs: from}), 1200)
	require.Equal(t, before, prepared(), "a large batch must not leave a prepared statement")
	require.Len(t, listBatch(ctx, t, st, store.RelationQuery{EntityIDs: from[:10]}), 20)
	require.Equal(t, before+1, prepared(), "control: a small batch is prepared and cached")
}
