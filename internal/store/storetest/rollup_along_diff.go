package storetest

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// alongVisits: a visits query lists each source's counted visits in id
// order, with the value each one supplies, beside the fold.
func alongVisits(t *testing.T, s store.Store) {
	seedGraphQueryEntities(t, s, "project", "PRJ-1")
	seedEntityWithProps(t, s, "action", "ACT-1", map[string]any{"status": "open", "n": 3})
	seedEntityWithProps(t, s, "action", "ACT-2", map[string]any{"status": "open", "n": 1})
	seedEntityWithProps(t, s, "action", "ACT-3", map[string]any{"status": "done", "n": 9})
	for _, id := range []string{"ACT-2", "ACT-1", "ACT-3"} {
		mustRel(t, s, id, "child_of", "PRJ-1")
	}
	q := store.RollupQuery{
		Sources: []string{"PRJ-1"}, Func: store.RollupMax, Value: "n", Numeric: true, Along: &store.AlongWalk{
			MaxDepth: 1, Visits: true,
			Segments: []store.AlongSegment{{Relation: "child_of", SourceIsTo: true, Through: map[string]store.AlongNode{
				"action": alongNode("action", alongWorld, new(alongStatus("open"))),
			}}},
		},
	}
	requireRollup(t, s, q, map[string]store.RollupResult{"PRJ-1": {Count: 2, Value: int64(3), Visits: []store.AlongVisit{
		{ID: "ACT-1", Type: "action"}, {ID: "ACT-2", Type: "action"},
	}}})
}

// alongDifferential compares the backend with [store.RollupGeneric] on
// random graphs of actions, projects and folders, for every fold, both
// directions, several depths and the visits form.
func alongDifferential(t *testing.T, s store.Store) {
	rng := rand.New(rand.NewPCG(7, 11))
	types := []string{"action", "project", "folder"}
	statuses := []string{"open", "done", "on_hold", "dropped"}
	var ids []string
	for i := range 40 {
		typ := types[i%len(types)]
		id := fmt.Sprintf("%s-%02d", typ[:3], i)
		props := map[string]any{"status": statuses[rng.IntN(len(statuses))], "label": fmt.Sprintf("L%02d", rng.IntN(50))}
		if rng.IntN(4) > 0 {
			props["n"] = rng.IntN(100) - 50
		}
		seedEntityWithProps(t, s, typ, id, props)
		ids = append(ids, id)
	}
	edges := map[[3]string]bool{}
	for range 90 {
		e := [3]string{ids[rng.IntN(len(ids))], []string{"child_of", "in_folder"}[rng.IntN(2)], ids[rng.IntN(len(ids))]}
		if e[0] != e[2] && !edges[e] {
			edges[e] = true
			mustRel(t, s, e[0], e[1], e[2])
		}
	}
	answered := 0
	for i := range 24 {
		q := alongHeldQuery(1+rng.IntN(8), alongWorld, ids[:20]...)
		q.Along.Segments[0].SourceIsTo = i%3 == 0
		q.Along.Segments[1].Repeat = i%2 == 0
		q.Along.Segments[1].Through["action"] = alongNode("action", alongWorld, new(alongStatus("done")))
		q.Along.Visits = i%4 == 1
		switch i % 5 {
		case 1:
			q.Func, q.Value, q.Numeric = store.RollupMin, "n", true
		case 2:
			q.Func, q.Value, q.Numeric = store.RollupMax, "n", true
		case 3:
			q.Func, q.Value = store.RollupMin, "label"
		case 4:
			q.Func, q.Value = store.RollupMax, "label"
		}
		got, err := store.Rollup(ctx(), s, q)
		require.NoError(t, err, "query %d", i)
		ref, err := store.RollupGeneric(ctx(), s, q)
		require.NoError(t, err, "query %d", i)
		require.Equal(t, ref, got, "query %d", i)
		answered += len(got)
	}
	require.Greater(t, answered, 24, "the random walks reach too little to compare")
}
