package dataentry

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestListPlanNarrowed_DeclinesOverLimits pins that a scope and a condition
// each within the store's limits, but over them together, take the Go path
// instead of a query every backend refuses (TKT-8HY05WB).
func TestListPlanNarrowed_DeclinesOverLimits(t *testing.T) {
	half := func() *store.Cond {
		args := make([]store.Cond, 0, store.MaxCondLeaves/2+1)
		for range store.MaxCondLeaves/2 + 1 {
			args = append(args, store.PropCond(store.PropPredicate{Property: "s", Op: store.PropEqual, Value: "x"}))
		}
		c := store.OrCond(args...)
		return &c
	}
	require.NoError(t, store.ValidateCond(*half()))

	plan, ok := listPlan{}.narrowed(store.GraphQuery{Where: half()})
	require.True(t, ok)
	_, ok = plan.narrowed(store.GraphQuery{Where: half()})
	require.False(t, ok, "the combined tree has more leaves than store.MaxCondLeaves")

	small := store.OrCond(store.PropCond(store.PropPredicate{Property: "s", Op: store.PropEqual, Value: "y"}))
	got, ok := plan.narrowed(store.GraphQuery{Where: &small})
	require.True(t, ok)
	require.Equal(t, store.CondAnd, got.query.Where.Op)
}
