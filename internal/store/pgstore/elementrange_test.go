package pgstore_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// rangeScope reads tracked tickets whose level is public, with secret visible where
// show is yes, and every tracks relation.
func rangeScope() store.ElementScope {
	eq := func(p, v string) store.PropPredicate {
		return store.PropPredicate{Property: p, Op: store.PropEqual, Value: v, Scalar: true}
	}
	return store.ElementScope{
		Entities: map[string]store.ElementTypeScope{
			"ticket": {
				Query: &store.GraphQuery{
					Props:      []store.PropPredicate{eq("level", "public")},
					HasInbound: &store.RelationPredicate{OfTypes: []string{"tracks"}},
				},
				Visibility: &store.FieldVisibility{Hidden: []store.HiddenField{{
					Property: "secret",
					Visible:  store.RowCondition{Any: []*store.GraphQuery{{Props: []store.PropPredicate{eq("show", "yes")}}}},
				}}},
			},
			"epic": {AllowAll: true, Visibility: &store.FieldVisibility{}},
		},
		Relations: map[string]store.RelationElementScope{"tracks": {Visibility: &store.FieldVisibility{}}},
	}
}

// seedRangeRows adds tickets up to n, each tracked by an epic every third.
func seedRangeRows(t *testing.T, s store.Store, from, n int) {
	t.Helper()
	ctx := context.Background()
	for i := from; i < n; i++ {
		e := entity.New(fmt.Sprintf("T-%05d", i), "ticket")
		e.SetString("level", []string{"public", "private"}[i%2])
		if i%3 == 0 {
			e.SetString("secret", "s")
		}
		if i%5 == 0 {
			e.SetString("show", "yes")
		}
		require.NoError(t, s.CreateEntity(ctx, e))
		if i%3 == 1 {
			epic := fmt.Sprintf("E-%05d", i)
			require.NoError(t, s.CreateEntity(ctx, entity.New(epic, "epic")))
			_, err := s.CreateRelation(ctx, entity.RelationKey{From: epic, Type: "tracks", To: e.ID}, nil)
			require.NoError(t, err)
		}
	}
}

// One range read is one statement whatever the number of rows, gated and
// with hidden fields (collection-reads budget).
func TestElementRange_OneStatementAtAnySize(t *testing.T) {
	// openWriter's pool carries the query tracer that counts statements.
	s := openWriter(t, freshFeedSchema(t))
	g, err := pgstore.NewElementRanges(s.(*pgstore.Store))
	require.NoError(t, err)

	seeded := 0
	for _, size := range []int{10, 50} {
		seedRangeRows(t, s, seeded, size)
		seeded = size
		for _, r := range []store.ElementRange{
			{Unbounded: true},
			{Lo: "ticket/T-00002", Hi: "ticket/T-00040"},
			{Relations: true, Unbounded: true},
		} {
			ctx, stats := store.WithQueryStats(context.Background())
			sum, err := g.RangeFingerprint(ctx, rangeScope(), r)
			require.NoError(t, err)
			require.NotZero(t, sum.Count)
			require.EqualValues(t, 1, stats.Queries(), "fingerprint of %+v at %d rows", r, size)

			ctx, stats = store.WithQueryStats(context.Background())
			_, err = g.ListRange(ctx, rangeScope(), r, store.RangeList{Limit: 100})
			require.NoError(t, err)
			require.EqualValues(t, 1, stats.Queries(), "list of %+v at %d rows", r, size)
		}
	}
}

// A row whose stored element is NULL is pending: counted, never summed or
// listed with a hash.
func TestElementRange_NullElementIsPending(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	g, err := pgstore.NewElementRanges(s)
	require.NoError(t, err)
	ctx := context.Background()
	seedElementRows(t, s)
	_, err = pool.Exec(ctx, `UPDATE entities SET element_hash = NULL WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE relations SET element_hash = NULL`)
	require.NoError(t, err)

	scope := store.ElementScope{
		Entities:  map[string]store.ElementTypeScope{"feature": {AllowAll: true, Visibility: &store.FieldVisibility{}}},
		Relations: map[string]store.RelationElementScope{"depends-on": {Visibility: &store.FieldVisibility{}}},
	}
	sum, err := g.RangeFingerprint(ctx, scope, store.ElementRange{Unbounded: true})
	require.NoError(t, err)
	require.Equal(t, uint64(2), sum.Count)
	require.Equal(t, uint64(1), sum.Pending)
	list, err := g.ListRange(ctx, scope, store.ElementRange{Unbounded: true}, store.RangeList{Limit: 10})
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.True(t, list[0].Pending)
	require.Empty(t, list[0].Hash)
	require.False(t, list[1].Pending)
	require.NotEmpty(t, list[1].Hash)

	sum, err = g.RangeFingerprint(ctx, scope, store.ElementRange{Relations: true, Unbounded: true})
	require.NoError(t, err)
	require.Equal(t, store.RangeSum{Count: 1, Pending: 1}, sum)
}

// A narrow range walks the element index instead of the whole table.
func TestElementRange_ExplainUsesElementIndex(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	seedRangeRows(t, s, 0, 3000)
	_, err = pool.Exec(context.Background(), "ANALYZE entities; ANALYZE relations")
	require.NoError(t, err)

	for _, tc := range []struct {
		r     store.ElementRange
		index string
	}{
		{store.ElementRange{Lo: "ticket/T-00100", Hi: "ticket/T-00120"}, "entities_element_idx"},
		{store.ElementRange{Relations: true, Lo: "E-00100", Hi: "E-00120"}, "relations_element_idx"},
	} {
		sqlText, args, err := pgstore.BuildRangeListSQLForTest(rangeScope(), tc.r, store.RangeList{Limit: 50})
		require.NoError(t, err)
		plan := explainRange(t, pool, sqlText, args)
		require.Contains(t, plan, tc.index, "range %+v does not use its element index:\n%s", tc.r, plan)
	}
}

func explainRange(t *testing.T, pool *pgxpool.Pool, sqlText string, args []any) string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "EXPLAIN "+sqlText, args...)
	require.NoError(t, err)
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		lines = append(lines, line)
	}
	require.NoError(t, rows.Err())
	return strings.Join(lines, "\n")
}
