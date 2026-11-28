package storetest

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
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
// directions, several depths and the visits form. A third of the entities
// also have a draft face, some values are extreme ints, each segment picks
// its own world and secret gate, a project counts differently on each
// segment, and some queries hit the row or hop cap. Every backend takes a
// visit's face, value and hit from its earliest arrival.
func alongDifferential(t *testing.T, s store.Store) {
	rng := rand.New(rand.NewPCG(7, 11))
	draft := mustFace(t, "draft")
	types := []string{"action", "project", "folder"}
	var ids []string
	for i := range 40 {
		typ := types[i%len(types)]
		id := fmt.Sprintf("%s-%02d", typ[:3], i)
		seedEntityWithProps(t, s, typ, id, alongDiffProps(rng))
		if rng.IntN(3) == 0 {
			e := entity.New(id, typ)
			e.Face = draft
			e.Properties = alongDiffProps(rng)
			require.NoError(t, s.CreateEntity(ctx(), e), "create %s at draft", id)
		}
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
	draftFirst := map[string]store.TypeResolution{}
	for _, typ := range types {
		draftFirst[typ] = store.TypeResolution{Chain: []entity.Face{draft, ""}, Fallback: store.FallbackExclude}
	}
	worlds := []store.FaceSelection{alongWorld, store.InWorld(store.NewWorldScope(draftFirst))}
	answered, over := 0, 0
	for i := range 48 {
		q := alongDiffQuery(rng, i, worlds, ids[:20])
		got, err := store.Rollup(ctx(), s, q)
		require.NoError(t, err, "query %d", i)
		ref, err := store.RollupGeneric(ctx(), s, q)
		require.NoError(t, err, "query %d", i)
		require.Equal(t, ref, got, "query %d", i)
		answered += len(got)
		for _, r := range got {
			if r.OverCap {
				over++
			}
		}
	}
	require.Greater(t, answered, 48, "the random walks reach too little to compare")
	require.Positive(t, over, "no query hit a cap")
}

// alongDiffQuery is the differential's query i: the probe's walk with a
// random direction, repeat, fold, world and secret gate per segment, a
// project node on the folder segment, and on every sixth query small caps.
func alongDiffQuery(rng *rand.Rand, i int, worlds []store.FaceSelection, srcs []string) store.RollupQuery {
	q := alongHeldQuery(1+rng.IntN(8), alongWorld, srcs...)
	q.Along.Segments[0].SourceIsTo = i%3 == 0
	q.Along.Segments[1].Repeat = i%2 == 0
	q.Along.Segments[1].Through["action"] = alongNode("action", alongWorld, new(alongStatus("done")))
	q.Along.Segments[1].Through["project"] = alongNode("project", alongWorld, new(alongStatus("open")))
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
	if i%6 == 5 {
		q.Along.RowCap, q.Along.HopCap = 3+rng.IntN(6), 1+rng.IntN(3)
	}
	for _, seg := range q.Along.Segments {
		faces, secret := worlds[rng.IntN(len(worlds))], rng.IntN(3) == 0
		for typ, n := range seg.Through {
			n.Gate.Faces = faces
			if secret {
				n.Gate.Props = []store.PropPredicate{{Property: "secret", Op: store.PropEqual, Value: "no", Scalar: true}}
			}
			seg.Through[typ] = n
		}
	}
	return q
}

// alongDiffProps draws one face's properties: a status, a label, a secret
// flag, and usually a number that is sometimes an extreme int.
func alongDiffProps(rng *rand.Rand) map[string]any {
	statuses := []string{"open", "done", "on_hold", "dropped"}
	extremes := []int64{math.MaxInt64, math.MinInt64, 1<<53 + 1, -(1<<53 + 1)}
	props := map[string]any{
		"status": statuses[rng.IntN(len(statuses))],
		"label":  fmt.Sprintf("L%02d", rng.IntN(50)),
		"secret": []string{"no", "yes"}[rng.IntN(2)],
	}
	switch rng.IntN(6) {
	case 0:
	case 1:
		props["n"] = extremes[rng.IntN(len(extremes))]
	default:
		props["n"] = rng.IntN(100) - 50
	}
	return props
}
