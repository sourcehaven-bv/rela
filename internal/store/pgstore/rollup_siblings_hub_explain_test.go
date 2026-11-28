//go:build postgres

package pgstore_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestRollupSiblingsExplainHubBounded: ACT-X sits in PRJ-H with 2,000
// hidden siblings, and ACT-Y is a child of 2,000 projects. Under a hop
// cap of 50 ne and se each read at most 51 edges per source, and both
// sources are over their cap.
func TestRollupSiblingsExplainHubBounded(t *testing.T) {
	const hopCap, hub = 50, 2000
	pool := newScopedPool(t)
	ctx := context.Background()
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	create := func(id, typ string) {
		e := entity.New(id, typ)
		e.Properties["status"] = "open"
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	link := func(from, to string) {
		_, lerr := s.CreateRelation(ctx, entity.RelationKey{From: from, Type: "child_of", To: to}, nil)
		require.NoError(t, lerr)
	}
	create("PRJ-H", "project")
	create("ACT-X", "action")
	create("ACT-Y", "action")
	link("ACT-X", "PRJ-H")
	for i := range hub {
		sib, prj := fmt.Sprintf("ACT-H%04d", i), fmt.Sprintf("PRJ-H%04d", i)
		create(sib, "action")
		link(sib, "PRJ-H")
		create(prj, "project")
		link("ACT-Y", prj)
	}
	_, err = pool.Exec(ctx, "ANALYZE entities; ANALYZE relations")
	require.NoError(t, err)
	q := store.RollupQuery{Sources: []string{"ACT-X", "ACT-Y"}, Func: store.RollupCount, Siblings: &store.SiblingWalk{
		Relation: "child_of", HopCap: hopCap,
		Parents: map[string]store.AlongNode{
			"project": {Gate: store.GraphQuery{EntityType: "project", Faces: store.InWorld(store.TrivialScope())}},
		},
		Siblings: map[string]store.AlongNode{"action": {Gate: alongPolicy("action")}},
	}}
	got, err := store.Rollup(ctx, s, q)
	require.NoError(t, err)
	require.Equal(t, map[string]store.RollupResult{"ACT-X": {OverCap: true}, "ACT-Y": {OverCap: true}}, got)
	plan := explainSiblings(t, pool, q)
	read := map[string]float64{}
	for _, sc := range plan.scans("relations") {
		read[sc.cte] += sc.node.ActualRows * sc.node.ActualLoops
	}
	t.Logf("relation rows per CTE %v", read)
	require.Positive(t, read["ne"])
	require.Positive(t, read["se"])
	for _, cte := range []string{"ne", "se"} {
		require.LessOrEqual(t, read[cte], float64(len(q.Sources)*(hopCap+1)), "%s reads a hub in full", cte)
	}
}
