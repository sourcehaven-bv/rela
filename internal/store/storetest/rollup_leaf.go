package storetest

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// runRollupLeafTests is the conformance suite for a roll-up leaf in a
// [store.Cond] ([store.RollupLeaf], design gap 3.5 step 2), run from
// [RunRollupTests]. A backend that renders the leaf natively and one that
// answers it in Go must both match the Go path's fold.
func runRollupLeafTests(t *testing.T, f Factory) {
	t.Helper()
	for name, run := range map[string]func(*testing.T, store.Store){
		"held":         leafHeld,
		"invalid":      leafInvalid,
		"paged":        leafPagedInherit,
		"differential": leafDifferential,
	} {
		t.Run(name, func(t *testing.T) { run(t, f(t)) })
	}
}

// leafHeld: ACT-1 sits in an on-hold project and ACT-2 in an active one;
// `held == true` matches ACT-1 only, `held == false` ACT-2 only, and the
// leaf composes with a property leaf.
func leafHeld(t *testing.T, s store.Store) {
	alongSeed(t, s, "action", "open", "ACT-1", "ACT-2")
	alongSeed(t, s, "project", "on_hold", "PRJ-1")
	alongSeed(t, s, "project", "active", "PRJ-2")
	mustRel(t, s, "ACT-1", "child_of", "PRJ-1")
	mustRel(t, s, "ACT-2", "child_of", "PRJ-2")
	held := alongHeldQuery(4, alongWorld)
	require.Equal(t, []string{"ACT-1"}, leafIDs(t, s, store.RollupLeaf(held, true)))
	require.Equal(t, []string{"ACT-2"}, leafIDs(t, s, store.RollupLeaf(held, false)))
	require.Empty(t, leafIDs(t, s, store.AndCond(store.RollupLeaf(held, false), alongStatus("done"))))
}

// leafPagedInherit: the project gate inherits its grant from the folder
// alice owns, and the project's id sorts before every action's. A store
// that reads the actions in keyset pages must not narrow the gate's
// closure by the page's resume id: with two-row pages, ACT-4 and ACT-5
// are on the second page and are held as ACT-1 and ACT-2 are.
func leafPagedInherit(t *testing.T, s store.Store) {
	alongSeed(t, s, "action", "open", "ACT-1", "ACT-2", "ACT-3", "ACT-4", "ACT-5", "ACT-6")
	alongSeed(t, s, "project", "on_hold", "AA-1")
	seedGraphQueryEntities(t, s, "folder", "FLD-1")
	seedGraphQueryEntities(t, s, "user", "alice")
	mustRel(t, s, "alice", "owns", "FLD-1")
	mustRel(t, s, "AA-1", "in_folder", "FLD-1")
	for _, id := range []string{"ACT-1", "ACT-2", "ACT-4", "ACT-5"} {
		mustRel(t, s, id, "child_of", "AA-1")
	}
	held := store.RollupQuery{Func: store.RollupCount, Along: &store.AlongWalk{
		MaxDepth: 2,
		Segments: []store.AlongSegment{{Relation: "child_of", Repeat: true, Through: map[string]store.AlongNode{
			"project": {Gate: store.GraphQuery{
				EntityType: "project", Faces: alongWorld,
				Related: []store.DirectedRelation{{Incoming: true, Pred: store.RelationPredicate{
					Endpoints: []string{"alice"}, OfTypes: []string{"owns"},
					EntityInheritThrough: []string{"in_folder"}, EntityDepth: 3,
				}}},
			}, Counts: new(alongStatus("on_hold"))},
		}}},
	}}
	require.Equal(t, []string{"ACT-1", "ACT-2", "ACT-4", "ACT-5"}, leafIDs(t, s, store.RollupLeaf(held, true)))
	require.Equal(t, []string{"ACT-3", "ACT-6"}, leafIDs(t, s, store.RollupLeaf(held, false)))
}

// leafInvalid: a leaf with sources, another fold, or visits is refused.
func leafInvalid(t *testing.T, s store.Store) {
	alongSeed(t, s, "action", "open", "ACT-1")
	for name, q := range map[string]store.RollupQuery{
		"sources": alongHeldQuery(2, alongWorld, "ACT-1"),
		"min":     func() store.RollupQuery { q := alongHeldQuery(2, alongWorld); q.Func = store.RollupMin; return q }(),
		"visits":  func() store.RollupQuery { q := alongHeldQuery(2, alongWorld); q.Along.Visits = true; return q }(),
		"no_walk": {Func: store.RollupCount},
	} {
		w := store.RollupLeaf(q, true)
		_, err := collectIDs(s, store.GraphQuery{EntityType: "action", Faces: alongWorld, Where: &w})
		require.ErrorIs(t, err, store.ErrInvalidQuery, name)
	}
}

// leafDifferential compares the pushed leaf with the Go path's fold
// ([store.RollupGeneric] over the candidates, then [store.RollupCond.Judge])
// on random trees with cycles, broken caps, faces, hidden entities and
// unset defaults. Each reader is a gate: one sees everything, one does not
// see the secret rows; each reads in the trivial world and a draft-first
// world. Both the whole result and an ordered page must agree.
func leafDifferential(t *testing.T, s store.Store) {
	ids := leafRandomTree(t, s, rand.New(rand.NewPCG(5, 9)))
	require.NotEmpty(t, ids)
	draft, err := entity.ParseFace("draft")
	require.NoError(t, err)
	worlds := map[string]store.FaceSelection{
		"trivial": alongWorld,
		"draft": store.InWorld(store.NewWorldScope(map[string]store.TypeResolution{
			"project": {Chain: []entity.Face{draft, ""}, Fallback: store.FallbackExclude},
			"action":  {Chain: []entity.Face{draft, ""}, Fallback: store.FallbackExclude},
		})),
	}
	matched := 0
	for wname, world := range worlds {
		for _, hidden := range []bool{false, true} {
			for qname, q := range leafQueries(world, hidden) {
				for _, holds := range []bool{true, false} {
					name := fmt.Sprintf("%s/%s/hidden=%v/holds=%v", wname, qname, hidden, holds)
					matched += leafCompare(t, s, name, leafCandidates(world, hidden), store.RollupLeaf(q, holds))
				}
			}
		}
	}
	require.Greater(t, matched, 40, "the random trees match too little to compare")
}

// leafCompare checks one leaf against the Go path and returns the number
// of rows it matched.
func leafCompare(t *testing.T, s store.Store, name string, base store.GraphQuery, leaf store.Cond) int {
	t.Helper()
	candidates, err := collectIDs(s, base)
	require.NoError(t, err, name)
	q := *leaf.Rollup
	q.Query.Sources = candidates
	res, err := store.RollupGeneric(ctx(), s, q.Query)
	require.NoError(t, err, name)
	var want []string
	for _, id := range candidates {
		if holds, known := q.Judge(res[id]); holds && known {
			want = append(want, id)
		}
	}
	pushed := base
	pushed.Where = &leaf
	got, err := collectIDs(s, pushed)
	require.NoError(t, err, name)
	require.ElementsMatch(t, want, got, name)
	page := pushed
	page.OrderBy, page.Limit = []store.OrderSpec{{Property: "label"}}, 5
	gotPage, err := collectIDs(s, page)
	require.NoError(t, err, name)
	require.Equal(t, leafFirstByLabel(t, s, want, 5), gotPage, name+" page")
	return len(got)
}

// leafCandidates is the list's own query: actions, the secret ones hidden
// when hidden.
func leafCandidates(world store.FaceSelection, hidden bool) store.GraphQuery {
	q := store.GraphQuery{EntityType: "action", Faces: world}
	if hidden {
		q.Props = []store.PropPredicate{leafNotSecret}
	}
	return q
}

var leafNotSecret = store.PropPredicate{Property: "secret", Op: store.PropEqual, Value: "no", Scalar: true}

// leafQueries are the probe's held walk and waiting siblings, with each
// gate hiding the secret rows when hidden.
func leafQueries(world store.FaceSelection, hidden bool) map[string]store.RollupQuery {
	held := alongHeldQuery(6, world)
	waiting := siblingsQuery(0)
	climbing := siblingsQuery(1)
	out := map[string]store.RollupQuery{"held": held, "waiting": waiting, "climbing": climbing}
	for _, q := range out {
		gates := []map[string]store.AlongNode{}
		if q.Along != nil {
			for _, seg := range q.Along.Segments {
				gates = append(gates, seg.Through)
			}
		} else {
			gates = append(gates, q.Siblings.Parents, q.Siblings.Siblings)
		}
		for _, through := range gates {
			for typ, n := range through {
				n.Gate.Faces = world
				if hidden {
					n.Gate.Props = []store.PropPredicate{leafNotSecret}
				}
				through[typ] = n
			}
		}
	}
	return out
}

// leafRandomTree seeds actions, projects and folders with random status,
// ordering, secrecy and labels, some with no status (a default to fill)
// and some with a draft face; then random child_of edges with random order
// values (cycles and several parents included) and in_folder edges. It
// returns the action ids.
func leafRandomTree(t *testing.T, s store.Store, rng *rand.Rand) []string {
	t.Helper()
	draft, err := entity.ParseFace("draft")
	require.NoError(t, err)
	statuses := []string{"open", "done", "on_hold", "dropped", ""}
	var actions, all []string
	for i := range 48 {
		typ := []string{"action", "action", "project", "folder"}[i%4]
		id := fmt.Sprintf("%s-%02d", typ[:3], i)
		props := map[string]any{"label": fmt.Sprintf("L%02d", rng.IntN(30)), "secret": "no"}
		if st := statuses[rng.IntN(len(statuses))]; st != "" {
			props["status"] = st
		}
		if rng.IntN(2) == 0 {
			props["ordering"] = "sequential"
		}
		if rng.IntN(6) == 0 {
			props["secret"] = "yes"
		}
		seedEntityWithProps(t, s, typ, id, props)
		if typ != "folder" && rng.IntN(5) == 0 {
			face := entity.New(id, typ)
			face.Face = draft
			face.Properties = map[string]any{"label": props["label"], "secret": "no",
				"status": statuses[rng.IntN(4)], "ordering": "sequential"}
			require.NoError(t, s.CreateEntity(ctx(), face))
		}
		if typ == "action" {
			actions = append(actions, id)
		}
		all = append(all, id)
	}
	seen := map[[3]string]bool{}
	for range 110 {
		rel := []string{"child_of", "child_of", "in_folder"}[rng.IntN(3)]
		e := [3]string{all[rng.IntN(len(all))], rel, all[rng.IntN(len(all))]}
		if e[0] == e[2] || seen[e] {
			continue
		}
		seen[e] = true
		var props map[string]any
		if rel == "child_of" && rng.IntN(5) > 0 {
			props = map[string]any{metamodel.OrderPropertyIn: []any{1, 2, 2.5, 3, 7}[rng.IntN(5)]}
		}
		mustRelProps(t, s, e[0], e[1], e[2], props)
	}
	return actions
}

// leafFirstByLabel is the first n of ids ordered by label, then id, as
// [store.GraphQuery.OrderBy] pages them.
func leafFirstByLabel(t *testing.T, s store.Store, ids []string, n int) []string {
	t.Helper()
	label := map[string]string{}
	for e, err := range s.GraphQuery(ctx(), store.GraphQuery{EntityType: "action", Faces: alongWorld}) {
		require.NoError(t, err)
		label[e.ID], _ = e.Properties["label"].(string)
	}
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, func(a, b string) int { return cmp.Or(cmp.Compare(label[a], label[b]), cmp.Compare(a, b)) })
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

// leafIDs returns the actions where holds, in id order.
func leafIDs(t *testing.T, s store.Store, where store.Cond) []string {
	t.Helper()
	ids, err := collectIDs(s, store.GraphQuery{EntityType: "action", Faces: alongWorld, Where: &where})
	require.NoError(t, err)
	slices.Sort(ids)
	return ids
}

// collectIDs runs q and returns the ids in the order the backend returns
// them, or the first error.
func collectIDs(s store.Store, q store.GraphQuery) ([]string, error) {
	var ids []string
	for e, err := range s.GraphQuery(ctx(), q) {
		if err != nil {
			return nil, err
		}
		ids = append(ids, e.ID)
	}
	return ids, nil
}
