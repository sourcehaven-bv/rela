package storetest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// runRollupCapTests is the conformance suite for the row cap of along and
// siblings roll-ups and leaves, run from [RunRollupTests]: each source is
// capped alone, only gated rows count, and an absent leaf fails closed
// under every polarity.
func runRollupCapTests(t *testing.T, f Factory) {
	t.Helper()
	for name, run := range map[string]func(*testing.T, store.Store){
		"along_per_source":    capAlongPerSource,
		"along_hidden":        capAlongHidden,
		"along_hop":           capAlongHop,
		"siblings_per_source": capSiblingsPerSource,
		"siblings_hidden":     capSiblingsHidden,
		"climb_hidden":        capClimbHidden,
		"leaf_fails_closed":   capLeafFailsClosed,
	} {
		t.Run(name, func(t *testing.T) { run(t, f(t)) })
	}
}

// capShown is the gate of a reader who does not see secret rows.
var capShown = store.PropPredicate{Property: "secret", Op: store.PropNotEqualOrEmpty, Value: "yes", Scalar: true}

// capSecret creates one secret entity per id with the given status.
func capSecret(t *testing.T, s store.Store, typ, status string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		seedEntityWithProps(t, s, typ, id, map[string]any{"status": status, "secret": "yes"})
	}
}

// capFanOut makes n entities of typ named prefix-i, each a parent of from.
func capFanOut(t *testing.T, s store.Store, from, typ, prefix string, n int, secret bool) {
	t.Helper()
	for i := range n {
		id := fmt.Sprintf("%s-%02d", prefix, i)
		if secret {
			capSecret(t, s, typ, "active", id)
		} else {
			alongSeed(t, s, typ, "active", id)
		}
		mustRel(t, s, from, "child_of", id)
	}
}

// capGateAlong hides the secret rows from every gate of q's walk.
func capGateAlong(q store.RollupQuery) store.RollupQuery {
	for _, seg := range q.Along.Segments {
		for typ, n := range seg.Through {
			n.Gate.Props = []store.PropPredicate{capShown}
			seg.Through[typ] = n
		}
	}
	return q
}

// capGateSiblings hides the secret rows from every gate of q's walk.
func capGateSiblings(q store.RollupQuery) store.RollupQuery {
	for _, nodes := range []map[string]store.AlongNode{q.Siblings.Parents, q.Siblings.Siblings} {
		for typ, n := range nodes {
			n.Gate.Props = []store.PropPredicate{capShown}
			nodes[typ] = n
		}
	}
	return q
}

// capAlongPerSource: ACT-1 reaches ten projects, ACT-2 one on-hold
// project. With a cap of three ACT-1 is over it alone; ACT-2 keeps its
// answer, in one call or alone.
func capAlongPerSource(t *testing.T, s store.Store) {
	alongSeed(t, s, "action", "open", "ACT-1", "ACT-2")
	alongSeed(t, s, "project", "on_hold", "PRJ-1")
	capFanOut(t, s, "ACT-1", "project", "PRJ-W", 10, false)
	mustRel(t, s, "ACT-2", "child_of", "PRJ-1")
	q := alongHeldQuery(4, alongWorld, "ACT-1", "ACT-2")
	q.Along.RowCap = 3
	requireRollup(t, s, q, map[string]store.RollupResult{"ACT-1": {OverCap: true}, "ACT-2": {Count: 1}})
	q.Sources = []string{"ACT-2"}
	requireRollup(t, s, q, map[string]store.RollupResult{"ACT-2": {Count: 1}})
}

// capAlongHidden: ACT-1 reaches ten secret projects and one visible
// on-hold project. A reader who sees no secret row holds one triple, so a
// cap of one passes; the reader who sees them all is over it.
func capAlongHidden(t *testing.T, s store.Store) {
	alongSeed(t, s, "action", "open", "ACT-1")
	alongSeed(t, s, "project", "on_hold", "PRJ-1")
	mustRel(t, s, "ACT-1", "child_of", "PRJ-1")
	capFanOut(t, s, "ACT-1", "project", "PRJ-S", 10, true)
	q := alongHeldQuery(4, alongWorld, "ACT-1")
	q.Along.RowCap = 1
	requireRollup(t, s, q, map[string]store.RollupResult{"ACT-1": {OverCap: true}})
	requireRollup(t, s, capGateAlong(q), alongOne())
}

// capAlongHop: PRJ-1 has ten secret children and ACT-1, a visible open
// one. Walking down from PRJ-1 reads eleven edges in one hop, so a hop cap
// of ten leaves PRJ-1 over its cap even for the reader who sees one child;
// a hop cap of eleven does not. PRJ-2, with one child, keeps its answer.
func capAlongHop(t *testing.T, s store.Store) {
	alongSeed(t, s, "project", "active", "PRJ-1", "PRJ-2")
	alongSeed(t, s, "action", "open", "ACT-1", "ACT-2")
	mustRel(t, s, "ACT-1", "child_of", "PRJ-1")
	mustRel(t, s, "ACT-2", "child_of", "PRJ-2")
	for i := range 10 {
		id := fmt.Sprintf("ACT-S%d", i)
		capSecret(t, s, "action", "open", id)
		mustRel(t, s, id, "child_of", "PRJ-1")
	}
	q := capGateAlong(store.RollupQuery{Sources: []string{"PRJ-1", "PRJ-2"}, Func: store.RollupCount,
		Along: &store.AlongWalk{MaxDepth: 2, HopCap: 10, Segments: []store.AlongSegment{{
			Relation: "child_of", SourceIsTo: true, Repeat: true, Through: map[string]store.AlongNode{
				"action": alongNode("action", alongWorld, new(alongStatus("open"))),
			},
		}}}})
	requireRollup(t, s, q, map[string]store.RollupResult{"PRJ-1": {OverCap: true}, "PRJ-2": {Count: 1}})
	q.Along.HopCap = 11
	requireRollup(t, s, q, map[string]store.RollupResult{"PRJ-1": {Count: 1}, "PRJ-2": {Count: 1}})
}

// capSiblingsPerSource: ACT-1 sits in PRJ-1 with five siblings, ACT-9 in
// PRJ-2 with one. With a cap of four ACT-1 is over it alone.
func capSiblingsPerSource(t *testing.T, s store.Store) {
	seedEntityWithProps(t, s, "project", "PRJ-1", map[string]any{"ordering": "sequential"})
	seedEntityWithProps(t, s, "project", "PRJ-2", map[string]any{"ordering": "sequential"})
	siblingsSeed(t, s, "PRJ-1", open("ACT-1", 9), open("ACT-2", 1), open("ACT-3", 2),
		open("ACT-4", 3), open("ACT-5", 4), open("ACT-6", 5))
	siblingsSeed(t, s, "PRJ-2", open("ACT-8", 1), open("ACT-9", 2))
	q := siblingsQuery(0, "ACT-1", "ACT-9")
	q.Siblings.RowCap = 4
	requireRollup(t, s, q, map[string]store.RollupResult{"ACT-1": {OverCap: true}, "ACT-9": behind(1, "PRJ-2")})
}

// capSiblingsHidden: ACT-1 sits in PRJ-1 after one visible open sibling
// and eight secret ones. The reader who sees no secret row reads three
// gated rows (ACT-1's edge, its own and the visible sibling's edge in), so
// a cap of three passes; the reader who sees them all is over it.
func capSiblingsHidden(t *testing.T, s store.Store) {
	seedEntityWithProps(t, s, "project", "PRJ-1", map[string]any{"ordering": "sequential"})
	siblingsSeed(t, s, "PRJ-1", open("ACT-1", 9), open("ACT-2", 1))
	for i := range 8 {
		id := fmt.Sprintf("ACT-S%d", i)
		capSecret(t, s, "action", "open", id)
		mustRelProps(t, s, id, "child_of", "PRJ-1", map[string]any{"_order_in": 0})
	}
	q := siblingsQuery(0, "ACT-1")
	q.Siblings.RowCap = 3
	requireRollup(t, s, q, map[string]store.RollupResult{"ACT-1": {OverCap: true}})
	requireRollup(t, s, capGateSiblings(q), map[string]store.RollupResult{"ACT-1": behind(1, "PRJ-1")})
}

// capClimbHidden: ACT-1's climb reaches GRP-1 and ten secret actions it
// is also a child of. The reader who sees no secret row holds three rows
// (the climb triple, ACT-1's edge into GRP-1 as a node and as a sibling),
// so a cap of three passes; the reader who sees them all is over it.
func capClimbHidden(t *testing.T, s store.Store) {
	seedEntityWithProps(t, s, "action", "GRP-1", map[string]any{"status": "open", "ordering": "sequential"})
	siblingsSeed(t, s, "GRP-1", open("ACT-1", 1))
	for i := range 10 {
		id := fmt.Sprintf("GRP-S%d", i)
		seedEntityWithProps(t, s, "action", id, map[string]any{"status": "open", "secret": "yes"})
		mustRel(t, s, "ACT-1", "child_of", id)
	}
	q := siblingsQuery(1, "ACT-1")
	q.Siblings.RowCap = 3
	requireRollup(t, s, q, map[string]store.RollupResult{"ACT-1": {OverCap: true}})
	requireRollup(t, s, capGateSiblings(q), map[string]store.RollupResult{"ACT-1": behind(0, "GRP-1")})
}

// capLeafFailsClosed: ACT-1 is over the cap, ACT-2 is held and ACT-3 is
// not. ACT-1 matches neither polarity, nor the negation of either, nor an
// OR with a true leaf; ACT-2 and ACT-3 keep their answers.
func capLeafFailsClosed(t *testing.T, s store.Store) {
	alongSeed(t, s, "action", "open", "ACT-1", "ACT-2", "ACT-3")
	alongSeed(t, s, "project", "on_hold", "PRJ-1")
	alongSeed(t, s, "project", "active", "PRJ-2")
	capFanOut(t, s, "ACT-1", "project", "PRJ-W", 10, false)
	mustRel(t, s, "ACT-2", "child_of", "PRJ-1")
	mustRel(t, s, "ACT-3", "child_of", "PRJ-2")
	held := alongHeldQuery(4, alongWorld)
	held.Along.RowCap = 3
	yes, no := store.RollupLeaf(held, true), store.RollupLeaf(held, false)
	for name, tc := range map[string]struct {
		where store.Cond
		want  []string
	}{
		"held":          {yes, []string{"ACT-2"}},
		"not_held":      {no, []string{"ACT-3"}},
		"not_of_held":   {store.NotCond(yes), []string{"ACT-3"}},
		"not_of_not":    {store.NotCond(no), []string{"ACT-2"}},
		"or_true":       {store.OrCond(alongStatus("open"), yes), []string{"ACT-2", "ACT-3"}},
		"and_with_prop": {store.AndCond(alongStatus("open"), store.NotCond(yes)), []string{"ACT-3"}},
	} {
		require.Equal(t, tc.want, leafIDs(t, s, tc.where), name)
		n, err := store.CountMatched(ctx(), s, store.GraphQuery{EntityType: "action", Faces: alongWorld, Where: &tc.where})
		require.NoError(t, err, name)
		require.Equal(t, len(tc.want), n, name+" count")
	}
}
