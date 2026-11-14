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

// seedBatchEdges creates n "contains" edges P-i -> C-i and returns both id
// lists.
func seedBatchEdges(t *testing.T, st store.Store, n int) (from, to []string) {
	t.Helper()
	ctx := context.Background()
	for i := range n {
		key := entity.RelationKey{From: fmt.Sprintf("P-%04d", i), Type: "contains", To: fmt.Sprintf("C-%04d", i)}
		_, err := st.CreateRelation(ctx, key, &store.RelationData{})
		require.NoError(t, err)
		from, to = append(from, key.From), append(to, key.To)
	}
	return from, to
}

// An id batch is sent once, not once per page (TKT-K7EM7Z5). At the fixed
// page size the gantt's edge read took n/500 statements that each carried
// every id, so it grew with the square of the chart.
func TestListRelations_IDBatchReadsInOneStatement(t *testing.T) {
	st := openWriter(t, freshFeedSchema(t))
	pgstore.SetIteratorPageSizeForTest(t, 10)
	from, to := seedBatchEdges(t, st, 120)
	for _, n := range []int{30, 120} {
		ctx, stats := store.WithQueryStats(context.Background())
		q := store.RelationQuery{
			Type: "contains", Direction: store.DirectionOutgoing, EntityIDs: from[:n], PeerIDs: to[:n],
		}
		got := 0
		for _, err := range st.ListRelations(ctx, q) {
			require.NoError(t, err)
			got++
		}
		require.Equal(t, n, got)
		require.EqualValues(t, 1, stats.Queries(), "statements for a batch of %d edges", n)
	}
}

// A large id batch runs as an unnamed statement, so PostgreSQL plans it for
// the actual arrays and never reuses a generic plan for it. A generic plan
// compares every row with every element of `to_id = ANY($n)`; on the gantt's
// edge read at 57k entities that took 8.4 s instead of 33 ms.
//
// The pool has one connection, so pg_prepared_statements counts what the
// store's statements left behind on it. The small batch is the control: it
// proves a cached statement would show up there.
func TestListRelations_LargeBatchIsNotPrepared(t *testing.T) {
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

	from, to := seedBatchEdges(t, st, 300)
	prepared := func() int {
		t.Helper()
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_prepared_statements`).Scan(&n))
		return n
	}
	list := func(n int) {
		t.Helper()
		q := store.RelationQuery{
			Type: "contains", Direction: store.DirectionOutgoing, EntityIDs: from[:n], PeerIDs: to[:n],
		}
		got := 0
		for _, err := range st.ListRelations(ctx, q) {
			require.NoError(t, err)
			got++
		}
		require.Equal(t, n, got)
	}

	prepared() // the count statement itself is cached on its first run
	before := prepared()
	list(300) // 600 ids: over the one-shot threshold
	require.Equal(t, before, prepared(), "a large batch must not leave a prepared statement")
	list(10) // 20 ids: the cached statement
	require.Equal(t, before+1, prepared(), "control: a small batch is prepared and cached")
}
