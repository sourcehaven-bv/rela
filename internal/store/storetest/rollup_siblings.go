package storetest

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// runRollupSiblingsTests is the conformance suite for an earlier-siblings
// roll-up ([store.SiblingWalk], design gap 2), run from [RunRollupTests].
// Every case runs the backend's answer and [store.RollupGeneric]. Each
// scenario's doc says what it pins for a backend author.
func runRollupSiblingsTests(t *testing.T, f Factory) {
	t.Helper()
	for name, run := range map[string]func(*testing.T, store.Store){
		"waiting":       siblingsWaiting,
		"default_fill":  siblingsDefaultFill,
		"order_values":  siblingsOrderValues,
		"climb":         siblingsClimb,
		"drag_reorder":  siblingsDragReorder,
		"several_edges": siblingsSeveralEdges,
		"gates":         siblingsGates,
		"row_cap":       siblingsRowCap,
		"invalid":       siblingsInvalid,
	} {
		t.Run(name, func(t *testing.T) { run(t, f(t)) })
	}
}

func siblingsProp(prop, v string, orEmpty bool) store.Cond {
	op := store.PropEqual
	if orEmpty {
		op = store.PropEqualOrEmpty
	}
	return store.PropLeaf(store.PropPredicate{Property: prop, Op: op, Value: v, Scalar: true})
}

// siblingsQuery is the probe's waiting roll-up over child_of: an open
// sibling (status open or unset) before the node, under an action or
// project parent whose ordering is sequential.
func siblingsQuery(climb int, srcs ...string) store.RollupQuery {
	sequential := siblingsProp("ordering", "sequential", false)
	open := siblingsProp("status", "open", true)
	return store.RollupQuery{Sources: srcs, Func: store.RollupCount, Siblings: &store.SiblingWalk{
		Relation: "child_of", Climb: climb,
		Parents: map[string]store.AlongNode{
			"action":  alongNode("action", alongWorld, &sequential),
			"project": alongNode("project", alongWorld, &sequential),
		},
		Siblings: map[string]store.AlongNode{"action": alongNode("action", alongWorld, &open)},
	}}
}

// siblingsCounting is siblingsQuery counting the open siblings before.
func siblingsCounting(srcs ...string) store.RollupQuery {
	q := siblingsQuery(0, srcs...)
	q.Siblings.CountSiblings = true
	return q
}

// siblingsSeed creates actions with a status, each a child of parent at
// the order value given; a nil order leaves _order_in unset.
func siblingsSeed(t *testing.T, s store.Store, parent string, children ...siblingChild) {
	t.Helper()
	for _, c := range children {
		seedEntityWithProps(t, s, "action", c.id, map[string]any{"status": c.status})
		props := map[string]any{}
		if c.order != nil {
			props[metamodel.OrderPropertyIn] = c.order
		}
		mustRelProps(t, s, c.id, "child_of", parent, props)
	}
}

type siblingChild struct {
	id, status string
	order      any
}

func open(id string, order any) siblingChild { return siblingChild{id, "open", order} }

func behind(n int, parents ...string) store.RollupResult {
	return store.RollupResult{Count: n, Parents: parents}
}

// siblingsWaiting: in a sequential project the first open action is not
// waiting and the second and third are; a completed first action does not
// block; a parallel project's children have no qualifying parent and no
// result. Counting, the third action has two open siblings before it.
func siblingsWaiting(t *testing.T, s store.Store) {
	seedEntityWithProps(t, s, "project", "PRJ-1", map[string]any{"ordering": "sequential"})
	seedEntityWithProps(t, s, "project", "PRJ-2", map[string]any{"ordering": "sequential"})
	seedEntityWithProps(t, s, "project", "PRJ-3", map[string]any{"ordering": "parallel"})
	siblingsSeed(t, s, "PRJ-1", open("ACT-1", 1), open("ACT-2", 2), open("ACT-3", 3))
	siblingsSeed(t, s, "PRJ-2", siblingChild{"ACT-4", "done", 1}, open("ACT-5", 2))
	siblingsSeed(t, s, "PRJ-3", open("ACT-6", 1), open("ACT-7", 2))
	srcs := []string{"ACT-1", "ACT-2", "ACT-3", "ACT-4", "ACT-5", "ACT-6", "ACT-7", "ACT-8"}
	requireRollup(t, s, siblingsQuery(0, srcs...), map[string]store.RollupResult{
		"ACT-1": behind(0, "PRJ-1"), "ACT-2": behind(1, "PRJ-1"), "ACT-3": behind(1, "PRJ-1"),
		"ACT-4": behind(0, "PRJ-2"), "ACT-5": behind(0, "PRJ-2"),
	})
	requireRollup(t, s, siblingsCounting("ACT-3", "ACT-5"), map[string]store.RollupResult{
		"ACT-3": behind(2, "PRJ-1"), "ACT-5": behind(0, "PRJ-2"),
	})
}

// siblingsDefaultFill: a parent with no stored ordering is not sequential,
// and a sibling with no stored status counts as open (the or-unset form a
// declared default is filled in with).
func siblingsDefaultFill(t *testing.T, s store.Store) {
	seedGraphQueryEntities(t, s, "project", "PRJ-1")
	seedEntityWithProps(t, s, "project", "PRJ-2", map[string]any{"ordering": "sequential"})
	siblingsSeed(t, s, "PRJ-1", open("ACT-1", 1), open("ACT-2", 2))
	seedGraphQueryEntities(t, s, "action", "ACT-3")
	mustRelProps(t, s, "ACT-3", "child_of", "PRJ-2", map[string]any{metamodel.OrderPropertyIn: 1})
	siblingsSeed(t, s, "PRJ-2", open("ACT-4", 2))
	requireRollup(t, s, siblingsQuery(0, "ACT-2", "ACT-4"), map[string]store.RollupResult{
		"ACT-4": behind(1, "PRJ-2"),
	})
}

// siblingsOrderValues: every kind of stored order value sorts as
// [metamodel.CompareOrderKeys] sorts the edges the backend's ListRelations
// returns, and none raises an error: unset and non-number values last,
// equal values tied on source id in byte order, fractions as float64, and
// numbers outside the float64 range, below its smallest subnormal, and
// near its largest finite value. Each action's count is its rank.
func siblingsOrderValues(t *testing.T, s store.Store) {
	seedEntityWithProps(t, s, "project", "PRJ-1", map[string]any{"ordering": "sequential"})
	values := []any{
		nil, 2, 2, 1.5, 1.25, -3, "3", true, map[string]any{"a": 1},
		json.Number("1e400"), json.Number("-1e400"), json.Number("1e-400"), json.Number("-1e-400"),
		json.Number("5e-324"), json.Number("2e-324"), json.Number("3e-324"),
		json.Number("1.7976931348623157e308"), json.Number("1.7976931348623158e308"),
		json.Number("1.797693134862315807e308"), json.Number("1.7976931348623159e308"),
		json.Number("-1.7976931348623158e308"), json.Number("12345678901234567890"), json.Number("0.1"),
		json.Number("9007199254740993"), json.Number("9007199254740992"),
	}
	ids := make([]string, len(values))
	children := make([]siblingChild, len(values))
	for i, v := range values {
		ids[i] = fmt.Sprintf("ACT-%02d", len(values)-i)
		children[i] = open(ids[i], v)
	}
	siblingsSeed(t, s, "PRJ-1", children...)
	var keys []metamodel.OrderKey
	for r, err := range s.ListRelations(ctx(), store.RelationQuery{
		Type: "child_of", EntityIDs: []string{"PRJ-1"}, Direction: store.DirectionIncoming,
	}) {
		require.NoError(t, err)
		keys = append(keys, metamodel.OrderKey{Value: r.Properties[metamodel.OrderPropertyIn], Peer: r.From})
	}
	require.Len(t, keys, len(values))
	slices.SortFunc(keys, metamodel.CompareOrderKeys)
	want := map[string]store.RollupResult{}
	for rank, k := range keys {
		want[k.Peer] = behind(rank, "PRJ-1")
	}
	requireRollup(t, s, siblingsCounting(ids...), want)
}

// siblingsClimb: in a sequential project, GRP-1 comes first and GRP-2
// second. A child of GRP-2 waits, though GRP-2 is parallel, because GRP-2
// is behind; a child of GRP-1 does not. ACT-4 is two hops below GRP-2, so
// it waits with a climb of 2 and not with a climb of 1.
func siblingsClimb(t *testing.T, s store.Store) {
	seedEntityWithProps(t, s, "project", "PRJ-1", map[string]any{"ordering": "sequential"})
	siblingsSeed(t, s, "PRJ-1", open("GRP-1", 1), open("GRP-2", 2))
	siblingsSeed(t, s, "GRP-1", open("ACT-1", 1))
	siblingsSeed(t, s, "GRP-2", open("ACT-2", 1))
	siblingsSeed(t, s, "ACT-2", open("ACT-4", 1))
	requireRollup(t, s, siblingsQuery(2, "ACT-1", "ACT-2", "ACT-4"), map[string]store.RollupResult{
		"ACT-1": behind(0, "PRJ-1"), "ACT-2": behind(1, "PRJ-1"), "ACT-4": behind(1, "PRJ-1"),
	})
	requireRollup(t, s, siblingsQuery(1, "ACT-4"), map[string]store.RollupResult{})
	requireRollup(t, s, siblingsQuery(0, "ACT-2"), map[string]store.RollupResult{})
}

// siblingsDragReorder: moving ACT-3 to the top is one write to its edge's
// order value. The next read has ACT-3 not waiting and ACT-1 waiting.
func siblingsDragReorder(t *testing.T, s store.Store) {
	seedEntityWithProps(t, s, "project", "PRJ-1", map[string]any{"ordering": "sequential"})
	siblingsSeed(t, s, "PRJ-1", open("ACT-1", 1), open("ACT-2", 2), open("ACT-3", 3))
	_, err := s.UpdateRelation(ctx(), entity.RelationKey{From: "ACT-3", Type: "child_of", To: "PRJ-1"},
		store.RelationData{Properties: map[string]any{metamodel.OrderPropertyIn: 0}})
	require.NoError(t, err)
	requireRollup(t, s, siblingsQuery(0, "ACT-1", "ACT-2", "ACT-3"), map[string]store.RollupResult{
		"ACT-1": behind(1, "PRJ-1"), "ACT-2": behind(1, "PRJ-1"), "ACT-3": behind(0, "PRJ-1"),
	})
}

// siblingsSeveralEdges: ACT-3 has two child_of edges, data that breaks the
// to-one cap. It is first under PRJ-1 and second under PRJ-2, so it is
// behind under one parent and waits.
func siblingsSeveralEdges(t *testing.T, s store.Store) {
	seedEntityWithProps(t, s, "project", "PRJ-1", map[string]any{"ordering": "sequential"})
	seedEntityWithProps(t, s, "project", "PRJ-2", map[string]any{"ordering": "sequential"})
	siblingsSeed(t, s, "PRJ-1", open("ACT-3", 1), open("ACT-4", 2))
	siblingsSeed(t, s, "PRJ-2", open("ACT-1", 1))
	mustRelProps(t, s, "ACT-3", "child_of", "PRJ-2", map[string]any{metamodel.OrderPropertyIn: 2})
	requireRollup(t, s, siblingsQuery(0, "ACT-3"), map[string]store.RollupResult{
		"ACT-3": behind(1, "PRJ-1", "PRJ-2"),
	})
}

// siblingsGates: a sibling the Siblings gate does not admit is no sibling,
// and a parent the Parents gate does not admit puts no node behind. With a
// climb, a hidden group ends the climb.
func siblingsGates(t *testing.T, s store.Store) {
	seedEntityWithProps(t, s, "project", "PRJ-1", map[string]any{"ordering": "sequential"})
	seedEntityWithProps(t, s, "project", "PRJ-2", map[string]any{"ordering": "sequential", "status": "hidden"})
	siblingsSeed(t, s, "PRJ-1", siblingChild{"ACT-1", "hidden", 1}, open("ACT-2", 2))
	siblingsSeed(t, s, "PRJ-1", siblingChild{"GRP-1", "hidden", 0})
	siblingsSeed(t, s, "GRP-1", open("ACT-3", 1))
	siblingsSeed(t, s, "PRJ-2", open("ACT-4", 1), open("ACT-5", 2))
	shown := []store.PropPredicate{{Property: "status", Op: store.PropNotEqualOrEmpty, Value: "hidden", Scalar: true}}
	q := siblingsQuery(3, "ACT-2", "ACT-3", "ACT-5")
	for _, nodes := range []map[string]store.AlongNode{q.Siblings.Parents, q.Siblings.Siblings} {
		for typ, n := range nodes {
			n.Gate.Props = shown
			nodes[typ] = n
		}
	}
	requireRollup(t, s, q, map[string]store.RollupResult{"ACT-2": behind(0, "PRJ-1")})
}

// siblingsRowCap: the walk reads ACT-1's edge and the three edges into
// PRJ-1; a cap of four passes and one of three fails closed.
func siblingsRowCap(t *testing.T, s store.Store) {
	seedEntityWithProps(t, s, "project", "PRJ-1", map[string]any{"ordering": "sequential"})
	siblingsSeed(t, s, "PRJ-1", open("ACT-1", 3), open("ACT-2", 2), open("ACT-3", 1))
	q := siblingsQuery(0, "ACT-1")
	q.Siblings.RowCap = 4
	requireRollup(t, s, q, map[string]store.RollupResult{"ACT-1": behind(1, "PRJ-1")})
	q.Siblings.RowCap = 3
	_, err := store.Rollup(ctx(), s, q)
	require.ErrorIs(t, err, store.ErrAlongRowCap)
	_, err = store.RollupGeneric(ctx(), s, q)
	require.ErrorIs(t, err, store.ErrAlongRowCap)
}

// siblingsInvalid: a query the backends would answer differently is
// refused before any read.
func siblingsInvalid(t *testing.T, s store.Store) {
	walk := func(mut func(*store.RollupQuery)) store.RollupQuery {
		q := siblingsQuery(0, "ACT-1")
		mut(&q)
		return q
	}
	for name, q := range map[string]store.RollupQuery{
		"with_relation":  walk(func(q *store.RollupQuery) { q.Relation = "child_of" }),
		"no_relation":    walk(func(q *store.RollupQuery) { q.Siblings.Relation = "" }),
		"climb_33":       walk(func(q *store.RollupQuery) { q.Siblings.Climb = store.MaxAlongDepth + 1 }),
		"min":            walk(func(q *store.RollupQuery) { q.Func, q.Value = store.RollupMin, "n" }),
		"count_climbing": walk(func(q *store.RollupQuery) { q.Siblings.Climb, q.Siblings.CountSiblings = 2, true }),
		"gate_type": walk(func(q *store.RollupQuery) {
			q.Siblings.Parents["project"] = alongNode("action", alongWorld, nil)
		}),
	} {
		_, err := store.Rollup(ctx(), s, q)
		require.ErrorIs(t, err, store.ErrInvalidQuery, name)
	}
}
