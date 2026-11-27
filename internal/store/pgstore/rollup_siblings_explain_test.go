//go:build postgres

package pgstore_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// siblingsPolicyQuery is the waiting roll-up over the along fixture's page,
// climbing three hops: every parent and sibling gated by [alongPolicy], a
// sibling counting when it is open.
func siblingsPolicyQuery(srcs ...string) store.RollupQuery {
	open := alongPolicyStatus("open")
	return store.RollupQuery{Sources: srcs, Func: store.RollupCount, Siblings: &store.SiblingWalk{
		Relation: "child_of", Climb: 3,
		Parents: map[string]store.AlongNode{
			"action": {Gate: alongPolicy("action")}, "project": {Gate: alongPolicy("project")},
		},
		Siblings: map[string]store.AlongNode{"action": {Gate: alongPolicy("action"), Counts: &open}},
	}}
}

// explainSiblings runs q's statement under EXPLAIN (ANALYZE, BUFFERS).
func explainSiblings(t *testing.T, pool *pgxpool.Pool, q store.RollupQuery) alongPlan {
	t.Helper()
	sqlText, args, err := pgstore.BuildSiblingsSQLForTest(q)
	require.NoError(t, err)
	var raw []byte
	require.NoError(t, pool.QueryRow(context.Background(),
		"EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+sqlText, args...).Scan(&raw))
	var plans []alongPlan
	require.NoError(t, json.Unmarshal(raw, &plans))
	require.Len(t, plans, 1)
	return plans[0]
}

// TestRollupSiblingsExplainBudget pins design section 4.4's plan checks on
// the along fixture: the page's rows and buffers do not grow with the
// padding, and no gate scans entities sequentially or without the ids it
// is seeded by. The page's four actions share one parent, so all but the
// first wait. The pads start at 3000: below that the planner seq-scans the
// small relations table, which is cheaper there.
func TestRollupSiblingsExplainBudget(t *testing.T) {
	plans := map[int]alongPlan{}
	for _, pad := range []int{3000, 9000} {
		pool := newScopedPool(t)
		seedAlongFixture(t, pool, pad)
		s, err := pgstore.New(pool)
		require.NoError(t, err)
		got, err := store.Rollup(context.Background(), s, siblingsPolicyQuery(alongPage...))
		require.NoError(t, err)
		parents := []string{"GRP-P1", "PRJ-P1"}
		require.Equal(t, map[string]store.RollupResult{
			"ACT-P1": {Count: 0, Parents: parents}, "ACT-P2": {Count: 1, Parents: parents},
			"ACT-P3": {Count: 1, Parents: parents}, "ACT-P4": {Count: 1, Parents: parents},
		}, got)
		plans[pad] = explainSiblings(t, pool, siblingsPolicyQuery(alongPage...))
	}
	small, large := plans[3000], plans[9000]
	require.InDelta(t, small.rows(), large.rows(), small.rows()*0.05, "actual rows grow with the padding")
	require.InDelta(t, small.buffers(), large.buffers(), small.buffers()*0.1, "buffers grow with the padding")
	for _, sc := range large.scans("entities") {
		require.NotEqual(t, "Seq Scan", sc.node.NodeType, "%s scans entities sequentially", sc.cte)
		require.Regexp(t, `\bid = `, sc.node.IndexCond, "%s scans entities by %s", sc.cte, sc.node.IndexCond)
	}
	for _, sc := range large.scans("relations") {
		require.NotEqual(t, "Seq Scan", sc.node.NodeType, "%s scans relations sequentially", sc.cte)
	}
}
