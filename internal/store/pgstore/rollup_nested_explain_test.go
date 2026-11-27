//go:build postgres

package pgstore_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// frontierQuery is the outer walk of a nested roll-up (design section 10.3)
// over the along fixture: the gated direct children of srcs, every one
// returned as a visit for Go to judge.
func frontierQuery(srcs ...string) store.RollupQuery {
	all := store.AndCond()
	return store.RollupQuery{Sources: srcs, Func: store.RollupCount, Along: &store.AlongWalk{
		MaxDepth: 1, Visits: true,
		Segments: []store.AlongSegment{{Relation: "child_of", SourceIsTo: true, Through: map[string]store.AlongNode{
			"action": {Gate: alongPolicy("action"), Counts: &all},
		}}},
	}}
}

// TestRollupNestedExplainBudget pins the composed statements of a nested
// roll-up on postgres: the frontier walk from GRP-P1, then one inner along
// and one inner siblings statement seeded by the frontier ids. Together
// their rows and buffers do not grow with the padding, and no gate scans
// entities sequentially. The pads start at 3000 for the reason
// [TestRollupSiblingsExplainBudget] gives.
func TestRollupNestedExplainBudget(t *testing.T) {
	type cost struct{ rows, buffers float64 }
	costs := map[int]cost{}
	var last []alongPlan
	for _, pad := range []int{3000, 9000} {
		pool := newScopedPool(t)
		seedAlongFixture(t, pool, pad)
		s, err := pgstore.New(pool)
		require.NoError(t, err)
		got, err := store.Rollup(context.Background(), s, frontierQuery("GRP-P1"))
		require.NoError(t, err)
		var frontier []string
		for _, v := range got["GRP-P1"].Visits {
			frontier = append(frontier, v.ID)
		}
		slices.Sort(frontier)
		require.Equal(t, alongPage, frontier)
		last = []alongPlan{
			explainAlong(t, pool, frontierQuery("GRP-P1")),
			explainAlong(t, pool, alongPolicyQuery(12, frontier...)),
			explainSiblings(t, pool, siblingsPolicyQuery(frontier...)),
		}
		var c cost
		for _, p := range last {
			c.rows += p.rows()
			c.buffers += p.buffers()
		}
		costs[pad] = c
	}
	small, large := costs[3000], costs[9000]
	t.Logf("rows %v -> %v, buffers %v -> %v", small.rows, large.rows, small.buffers, large.buffers)
	require.InDelta(t, small.rows, large.rows, small.rows*0.05, "actual rows grow with the padding")
	require.InDelta(t, small.buffers, large.buffers, small.buffers*0.1, "buffers grow with the padding")
	for _, p := range last {
		for _, sc := range p.scans("entities") {
			require.NotEqual(t, "Seq Scan", sc.node.NodeType, "%s scans entities sequentially", sc.cte)
		}
	}
}
