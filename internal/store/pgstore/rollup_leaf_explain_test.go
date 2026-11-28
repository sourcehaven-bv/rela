//go:build postgres

package pgstore_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// leafPageSize is the list page the leaf tests read.
const leafPageSize = 10

// leafListQuery is a list page of actions where `held == holds`: the
// probe's held walk as a roll-up leaf, every node gated by [alongPolicy].
// The candidates themselves are not gated by it: that gate's own closure
// reads the policy's whole tree, which is the list's cost with or without
// a leaf. Every action of the along fixture is held (an on-hold project or
// a dropped folder above it), so holds=true rejects no candidate and
// holds=false rejects every one.
func leafListQuery(holds bool) store.GraphQuery {
	q := store.GraphQuery{EntityType: "action", Faces: store.InWorld(store.TrivialScope())}
	held := alongPolicyQuery(12)
	leaf := store.RollupLeaf(held, holds)
	q.Where = &leaf
	q.Limit = leafPageSize
	return q
}

// explainLeaf runs q's page statement under EXPLAIN (ANALYZE, BUFFERS).
func explainLeaf(t *testing.T, pool *pgxpool.Pool, q store.GraphQuery) alongPlan {
	t.Helper()
	sqlText, args := pgstore.BuildGraphQuerySQLForTest(q, false)
	var raw []byte
	require.NoError(t, pool.QueryRow(context.Background(),
		"EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+sqlText, args...).Scan(&raw))
	var plans []alongPlan
	require.NoError(t, json.Unmarshal(raw, &plans))
	require.Len(t, plans, 1)
	return plans[0]
}

// leafRun is one padded store's measurements of both list pages.
type leafRun struct {
	cost                 map[bool]storetest.Cost
	plan                 map[bool]alongPlan
	accepted, candidates int
}

// measureLeaf seeds the along fixture with pad and reads both pages
// through [storetest.Counting], then explains them.
func measureLeaf(t *testing.T, pad int) leafRun {
	t.Helper()
	pool := newScopedPool(t)
	seedAlongFixture(t, pool, pad)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	run := leafRun{cost: map[bool]storetest.Cost{}, plan: map[bool]alongPlan{}}
	for _, holds := range []bool{true, false} {
		counting := storetest.NewCounting(s)
		var ids []string
		for e, err := range counting.GraphQuery(context.Background(), leafListQuery(holds)) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
		}
		if holds {
			require.Len(t, ids, leafPageSize)
			run.accepted = len(ids)
		} else {
			require.Empty(t, ids)
		}
		run.cost[holds] = counting.Cost()
		run.plan[holds] = explainLeaf(t, pool, leafListQuery(holds))
	}
	n, err := store.CountMatched(context.Background(), s, store.GraphQuery{
		EntityType: "action", Faces: store.InWorld(store.TrivialScope()),
	})
	require.NoError(t, err)
	run.candidates = n
	return run
}

// TestRollupLeafBudget pins the cost of a pushed list whose condition is a
// roll-up leaf (design section 8). The calls and rows returned are the
// same at stores of 100 and 1,000 entities. With few rejected candidates
// (holds=true here: none is rejected) the rows the plan reads are the same
// at 1,000 and 3,000 too; at 100 the planner rightly seq-scans the small
// tables, so that size is not compared. With many rejected candidates
// (holds=false: every one is rejected) the rows the plan reads grow with
// the candidates scanned: the cost of any filtered keyset scan, stated
// here rather than hidden. Here they grow faster than the candidates:
// alice owns every padding tree, and the policy gate reads her owner-of
// edges until it meets the candidate's tree, so a later tree costs more.
// The test counts rows, not time, so it lifts the statement timeout: the
// many-rejected scan at 3,000 entities passes 5 s under the race detector.
func TestRollupLeafBudget(t *testing.T) {
	pgstore.SetRollupStatementTimeoutForTest(t, 2*time.Minute)
	tiny, small, large := measureLeaf(t, 100), measureLeaf(t, 1000), measureLeaf(t, 3000)
	require.Equal(t, tiny.cost, small.cost, "calls or rows returned grow with the store")
	require.Equal(t, small.cost, large.cost, "calls or rows returned grow with the store")
	few, fewLarge := small.plan[true].rows(), large.plan[true].rows()
	t.Logf("few rejected: plan rows %v -> %v", few, fewLarge)
	require.InDelta(t, few, fewLarge, few*0.05, "few rejected: the plan's rows grow with the store")
	many, manyLarge := small.plan[false].rows(), large.plan[false].rows()
	t.Logf("many rejected: plan rows %v -> %v over %d -> %d candidates", many, manyLarge,
		small.candidates, large.candidates)
	require.Greater(t, manyLarge, many*float64(large.candidates)/float64(small.candidates)*0.5,
		"many rejected: the plan's rows should grow with the candidates scanned")
}

// TestRollupLeafExplain pins the leaf's plan at two padded sizes (1,000 and
// 3,000; below that the planner rightly seq-scans the small tables): the
// leaf runs inside the keyset scan, once per candidate scanned until the
// page fills rather than once over the type; the page's rows and buffers
// do not grow with the padding; and no scan of entities, under a gate or
// in the candidate scan, is sequential.
func TestRollupLeafExplain(t *testing.T) {
	plans := map[int]alongPlan{}
	for _, pad := range []int{1000, 3000} {
		pool := newScopedPool(t)
		seedAlongFixture(t, pool, pad)
		plans[pad] = explainLeaf(t, pool, leafListQuery(true))
	}
	small, large := plans[1000], plans[3000]
	t.Logf("rows %v -> %v, buffers %v -> %v", small.rows(), large.rows(), small.buffers(), large.buffers())
	require.Equal(t, "Limit", large.Plan.NodeType, "the page is not a limited scan")
	loops := leafSubplanLoops(large.Plan)
	require.NotZero(t, loops, "the leaf is not a subplan of the candidate scan")
	require.LessOrEqual(t, loops, float64(leafPageSize), "the leaf runs for more candidates than the page")
	require.InDelta(t, small.rows(), large.rows(), small.rows()*0.05, "actual rows grow with the padding")
	require.InDelta(t, small.buffers(), large.buffers(), small.buffers()*0.1, "buffers grow with the padding")
	for _, sc := range large.scans("entities") {
		require.NotEqual(t, "Seq Scan", sc.node.NodeType, "%s scans entities sequentially", sc.cte)
	}
	for _, sc := range large.scans("relations") {
		require.NotEqual(t, "Seq Scan", sc.node.NodeType, "%s scans relations sequentially", sc.cte)
	}
}

// leafSubplanLoops is the loops of the outermost SubPlan nodes under n,
// the leaf's: how many candidates it was evaluated for. Subplans inside it
// run per level and are not counted.
func leafSubplanLoops(n alongPlanNode) float64 {
	if strings.HasPrefix(n.Subplan, "SubPlan") {
		return n.ActualLoops
	}
	var most float64
	for _, c := range n.Plans {
		most = max(most, leafSubplanLoops(c))
	}
	return most
}
