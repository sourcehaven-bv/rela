package storetest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// runRollupAlongTests is the conformance suite for an inherited roll-up
// ([store.AlongWalk], design gap 1), run from [RunRollupTests]. Every case
// runs the backend's answer and [store.RollupGeneric]. Each scenario's doc
// says what it pins for a backend author.
func runRollupAlongTests(t *testing.T, f Factory) {
	t.Helper()
	for name, run := range map[string]func(*testing.T, store.Store){
		"held":                  alongHeld,
		"default_fill":          alongDefaultFill,
		"depth_cap":             alongDepthCap,
		"restart_once":          alongRestartOnce,
		"cycles":                alongCycles,
		"broken_caps":           alongBrokenCaps,
		"gate_breaks_paths":     alongGateBreaksPaths,
		"faces_and_worlds":      alongFacesAndWorlds,
		"tags":                  alongTags,
		"incoming_and_extremes": alongIncomingAndExtremes,
		"invalid":               alongInvalid,
		"visits":                alongVisits,
		"differential":          alongDifferential,
	} {
		t.Run(name, func(t *testing.T) { run(t, f(t)) })
	}
}

var alongWorld = store.InWorld(store.TrivialScope())

func alongStatus(v string) store.Cond {
	return store.PropLeaf(store.PropPredicate{Property: "status", Op: store.PropEqual, Value: v, Scalar: true})
}

func alongNode(typ string, faces store.FaceSelection, counts *store.Cond) store.AlongNode {
	return store.AlongNode{Gate: store.GraphQuery{EntityType: typ, Faces: faces}, Counts: counts}
}

func condPtr(c store.Cond) *store.Cond { return &c }

// alongHeldQuery is the probe's walk: [child_of*, in_folder*], counting an
// open action, an on-hold or dropped project, and a dropped folder.
func alongHeldQuery(depth int, faces store.FaceSelection, srcs ...string) store.RollupQuery {
	return store.RollupQuery{Sources: srcs, Func: store.RollupCount, Along: &store.AlongWalk{
		MaxDepth: depth,
		Segments: []store.AlongSegment{
			{Relation: "child_of", Repeat: true, Through: map[string]store.AlongNode{
				"action": alongNode("action", faces, condPtr(alongStatus("open"))),
				"project": alongNode("project", faces,
					condPtr(store.OrCond(alongStatus("on_hold"), alongStatus("dropped")))),
			}},
			{Relation: "in_folder", Repeat: true, Through: map[string]store.AlongNode{
				"folder": alongNode("folder", faces, condPtr(alongStatus("dropped"))),
			}},
		},
	}}
}

// alongChain links ids in order by rel.
func alongChain(t *testing.T, s store.Store, rel string, ids ...string) {
	t.Helper()
	for i := 0; i+1 < len(ids); i++ {
		mustRel(t, s, ids[i], rel, ids[i+1])
	}
}

// alongSeed creates one entity per id with the given status.
func alongSeed(t *testing.T, s store.Store, typ, status string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		seedEntityWithProps(t, s, typ, id, map[string]any{"status": status})
	}
}

func alongOne(src string) map[string]store.RollupResult {
	return map[string]store.RollupResult{src: {Count: 1}}
}

// alongHeld: ACT-1 in an active project is not held; ACT-2 in an on-hold
// one is; ACT-3 is under a folder dropped two levels up; ACT-4 is under an
// open group three levels up, below two done ones.
func alongHeld(t *testing.T, s store.Store) {
	alongSeed(t, s, "action", "open", "ACT-1", "ACT-2", "ACT-3", "ACT-4", "GRP-3")
	alongSeed(t, s, "action", "done", "GRP-1", "GRP-2")
	alongSeed(t, s, "project", "active", "PRJ-1", "PRJ-3")
	alongSeed(t, s, "project", "on_hold", "PRJ-2")
	alongSeed(t, s, "folder", "active", "FLD-1")
	alongSeed(t, s, "folder", "dropped", "FLD-2")
	alongChain(t, s, "child_of", "ACT-1", "PRJ-1")
	alongChain(t, s, "child_of", "ACT-2", "PRJ-2")
	alongChain(t, s, "child_of", "ACT-3", "PRJ-3")
	alongChain(t, s, "in_folder", "PRJ-3", "FLD-1", "FLD-2")
	alongChain(t, s, "child_of", "ACT-4", "GRP-1", "GRP-2", "GRP-3", "PRJ-1")
	requireRollup(t, s, alongHeldQuery(12, alongWorld, "ACT-1", "ACT-2", "ACT-3", "ACT-4"),
		map[string]store.RollupResult{"ACT-2": {Count: 1}, "ACT-3": {Count: 1}, "ACT-4": {Count: 1}})
}

// alongDefaultFill: a Counts that names the unset value ("") holds on a row
// that stores no status, the form a declared default is filled in with.
func alongDefaultFill(t *testing.T, s store.Store) {
	seedGraphQueryEntities(t, s, "action", "ACT-1")
	seedGraphQueryEntities(t, s, "project", "PRJ-1")
	mustRel(t, s, "ACT-1", "child_of", "PRJ-1")
	unset := store.PropLeaf(store.PropPredicate{Property: "status", Op: store.PropEqual, Value: ""})
	q := store.RollupQuery{Sources: []string{"ACT-1"}, Func: store.RollupCount, Along: &store.AlongWalk{
		MaxDepth: 2,
		Segments: []store.AlongSegment{{Relation: "child_of", Repeat: true, Through: map[string]store.AlongNode{
			"project": alongNode("project", alongWorld, condPtr(store.OrCond(alongStatus("active"), unset))),
		}}},
	}}
	requireRollup(t, s, q, alongOne("ACT-1"))
}

// alongDepthCap: PRJ-1 (on hold) is three hops above ACT-1. ACT-9 reaches
// ACT-2 directly and by a detour through ACT-8, so the earliest arrival
// puts PRJ-1 three hops above it too. Both are held at depth 3, neither at
// depth 2.
func alongDepthCap(t *testing.T, s store.Store) {
	alongSeed(t, s, "action", "done", "ACT-1", "ACT-2", "ACT-3", "ACT-8", "ACT-9")
	alongSeed(t, s, "project", "on_hold", "PRJ-1")
	alongChain(t, s, "child_of", "ACT-1", "ACT-2", "ACT-3", "PRJ-1")
	alongChain(t, s, "child_of", "ACT-9", "ACT-8", "ACT-2")
	mustRel(t, s, "ACT-9", "child_of", "ACT-2")
	requireRollup(t, s, alongHeldQuery(3, alongWorld, "ACT-1", "ACT-9"),
		map[string]store.RollupResult{"ACT-1": {Count: 1}, "ACT-9": {Count: 1}})
	requireRollup(t, s, alongHeldQuery(2, alongWorld, "ACT-1", "ACT-9"), map[string]store.RollupResult{})
}

// alongRestartOnce: PRJ-1 is reached at depth 1 and at depth 3, and the
// folder chain above it is walked once. The walk holds six triples (ACT-2,
// ACT-3 and PRJ-1 by child_of, FLD-1 to FLD-3 by in_folder): a row cap of
// six passes and one of five fails.
func alongRestartOnce(t *testing.T, s store.Store) {
	alongSeed(t, s, "action", "done", "ACT-1", "ACT-2", "ACT-3")
	alongSeed(t, s, "project", "active", "PRJ-1")
	alongSeed(t, s, "folder", "active", "FLD-1", "FLD-2")
	alongSeed(t, s, "folder", "dropped", "FLD-3")
	alongChain(t, s, "child_of", "ACT-1", "ACT-2", "ACT-3", "PRJ-1")
	mustRel(t, s, "ACT-1", "child_of", "PRJ-1")
	alongChain(t, s, "in_folder", "PRJ-1", "FLD-1", "FLD-2", "FLD-3")
	q := alongHeldQuery(12, alongWorld, "ACT-1")
	q.Along.RowCap = 6
	requireRollup(t, s, q, alongOne("ACT-1"))
	q.Along.RowCap = 5
	_, err := store.Rollup(ctx(), s, q)
	require.ErrorIs(t, err, store.ErrAlongRowCap)
	_, err = store.RollupGeneric(ctx(), s, q)
	require.ErrorIs(t, err, store.ErrAlongRowCap)
}

// alongCycles: ACT-1 and ACT-2 form a loop, as do ACT-3, ACT-4 and ACT-5;
// ACT-4 alone is done. The walks end, and a source is never its own visit.
func alongCycles(t *testing.T, s store.Store) {
	alongSeed(t, s, "action", "open", "ACT-1", "ACT-2", "ACT-3", "ACT-5")
	alongSeed(t, s, "action", "done", "ACT-4")
	alongChain(t, s, "child_of", "ACT-1", "ACT-2", "ACT-1")
	alongChain(t, s, "child_of", "ACT-3", "ACT-4", "ACT-5", "ACT-3")
	requireRollup(t, s, alongHeldQuery(store.MaxAlongDepth, alongWorld, "ACT-1", "ACT-2", "ACT-3", "ACT-4"),
		map[string]store.RollupResult{
			"ACT-1": {Count: 1}, "ACT-2": {Count: 1}, "ACT-3": {Count: 1}, "ACT-4": {Count: 2},
		})
}

// alongBrokenCaps: ACT-1 has two parents and PRJ-2 two folders, against the
// declared to-one caps; the walk follows every edge, and FLD-2 holds.
func alongBrokenCaps(t *testing.T, s store.Store) {
	alongSeed(t, s, "action", "done", "ACT-1")
	alongSeed(t, s, "project", "active", "PRJ-1", "PRJ-2")
	alongSeed(t, s, "folder", "active", "FLD-1")
	alongSeed(t, s, "folder", "dropped", "FLD-2")
	mustRel(t, s, "ACT-1", "child_of", "PRJ-1")
	mustRel(t, s, "ACT-1", "child_of", "PRJ-2")
	mustRel(t, s, "PRJ-2", "in_folder", "FLD-1")
	mustRel(t, s, "PRJ-2", "in_folder", "FLD-2")
	requireRollup(t, s, alongHeldQuery(4, alongWorld, "ACT-1"), alongOne("ACT-1"))
}

// alongGateBreaksPaths: a project the gate does not select is not visited,
// and the dropped folder above it is not reached.
func alongGateBreaksPaths(t *testing.T, s store.Store) {
	alongSeed(t, s, "action", "done", "ACT-1")
	seedEntityWithProps(t, s, "project", "PRJ-1", map[string]any{"status": "active", "secret": "yes"})
	alongSeed(t, s, "folder", "dropped", "FLD-1")
	mustRel(t, s, "ACT-1", "child_of", "PRJ-1")
	mustRel(t, s, "PRJ-1", "in_folder", "FLD-1")
	q := alongHeldQuery(4, alongWorld, "ACT-1")
	requireRollup(t, s, q, alongOne("ACT-1"))
	gate := q.Along.Segments[0].Through["project"]
	gate.Gate.Props = []store.PropPredicate{{Property: "secret", Op: store.PropEqual, Value: "no", Scalar: true}}
	q.Along.Segments[0].Through["project"] = gate
	requireRollup(t, s, q, map[string]store.RollupResult{})
}

// alongFacesAndWorlds: PRJ-1 is active at the prime face and on hold at
// draft. The trivial world judges the prime row, a draft-first world the
// draft row, and a gate that allows the prime face only never counts the
// draft.
func alongFacesAndWorlds(t *testing.T, s store.Store) {
	draft, err := entity.ParseFace("draft")
	require.NoError(t, err)
	alongSeed(t, s, "action", "done", "ACT-1")
	alongSeed(t, s, "project", "active", "PRJ-1")
	onHold := entity.New("PRJ-1", "project")
	onHold.Face = draft
	onHold.Properties["status"] = "on_hold"
	require.NoError(t, s.CreateEntity(ctx(), onHold))
	mustRel(t, s, "ACT-1", "child_of", "PRJ-1")
	draftFirst := store.InWorld(store.NewWorldScope(map[string]store.TypeResolution{
		"project": {Chain: []entity.Face{draft, ""}, Fallback: store.FallbackExclude},
	}))
	requireRollup(t, s, alongHeldQuery(2, alongWorld, "ACT-1"), map[string]store.RollupResult{})
	requireRollup(t, s, alongHeldQuery(2, draftFirst, "ACT-1"), alongOne("ACT-1"))
	q := alongHeldQuery(2, draftFirst, "ACT-1")
	gate := q.Along.Segments[0].Through["project"]
	gate.Gate.FaceIn = []entity.Face{""}
	q.Along.Segments[0].Through["project"] = gate
	requireRollup(t, s, q, map[string]store.RollupResult{})
}

// alongTags: [tagged, tag_parent*]. ACT-1 is tagged TAG-1, under the
// on-hold TAG-2; ACT-2 only TAG-3, whose parent is active.
func alongTags(t *testing.T, s store.Store) {
	seedGraphQueryEntities(t, s, "action", "ACT-1", "ACT-2")
	alongSeed(t, s, "tag", "active", "TAG-1", "TAG-3", "TAG-4")
	alongSeed(t, s, "tag", "on_hold", "TAG-2")
	mustRel(t, s, "ACT-1", "tagged", "TAG-1")
	mustRel(t, s, "ACT-1", "tagged", "TAG-3")
	mustRel(t, s, "ACT-2", "tagged", "TAG-3")
	mustRel(t, s, "TAG-1", "tag_parent", "TAG-2")
	mustRel(t, s, "TAG-3", "tag_parent", "TAG-4")
	tag := alongNode("tag", alongWorld, condPtr(alongStatus("on_hold")))
	q := store.RollupQuery{Sources: []string{"ACT-1", "ACT-2"}, Func: store.RollupCount, Along: &store.AlongWalk{
		MaxDepth: 4,
		Segments: []store.AlongSegment{
			{Relation: "tagged", Through: map[string]store.AlongNode{"tag": tag}},
			{Relation: "tag_parent", Repeat: true, Through: map[string]store.AlongNode{"tag": tag}},
		},
	}}
	requireRollup(t, s, q, alongOne("ACT-1"))
}

// alongIncomingAndExtremes: walked backwards, child_of reaches PRJ-1's
// children; min folds the counted ones only.
func alongIncomingAndExtremes(t *testing.T, s store.Store) {
	seedGraphQueryEntities(t, s, "project", "PRJ-1")
	seedEntityWithProps(t, s, "action", "ACT-1", map[string]any{"status": "open", "due": "2024-03-01"})
	seedEntityWithProps(t, s, "action", "ACT-2", map[string]any{"status": "done", "due": "2024-01-01"})
	mustRel(t, s, "ACT-1", "child_of", "PRJ-1")
	mustRel(t, s, "ACT-2", "child_of", "PRJ-1")
	q := store.RollupQuery{
		Sources: []string{"PRJ-1"}, Func: store.RollupMin, Value: "due", Along: &store.AlongWalk{
			MaxDepth: 1,
			Segments: []store.AlongSegment{{Relation: "child_of", SourceIsTo: true, Through: map[string]store.AlongNode{
				"action": alongNode("action", alongWorld, condPtr(store.AndCond())),
			}}},
		},
	}
	requireRollup(t, s, q, map[string]store.RollupResult{"PRJ-1": {Count: 2, Value: "2024-01-01"}})
	q.Along.Segments[0].Through["action"] = alongNode("action", alongWorld, condPtr(alongStatus("open")))
	requireRollup(t, s, q, map[string]store.RollupResult{"PRJ-1": {Count: 1, Value: "2024-03-01"}})
}

// alongInvalid: every backend refuses the same malformed walks.
func alongInvalid(t *testing.T, s store.Store) {
	walk := func(mut func(*store.RollupQuery)) store.RollupQuery {
		q := alongHeldQuery(2, alongWorld, "ACT-1")
		mut(&q)
		return q
	}
	for name, q := range map[string]store.RollupQuery{
		"with_relation": walk(func(q *store.RollupQuery) { q.Relation = "child_of" }),
		"no_segments":   walk(func(q *store.RollupQuery) { q.Along.Segments = nil }),
		"depth_zero":    walk(func(q *store.RollupQuery) { q.Along.MaxDepth = 0 }),
		"depth_33":      walk(func(q *store.RollupQuery) { q.Along.MaxDepth = store.MaxAlongDepth + 1 }),
		"sum":           walk(func(q *store.RollupQuery) { q.Func, q.Value = store.RollupSum, "n" }),
		"gate_type": walk(func(q *store.RollupQuery) {
			q.Along.Segments[1].Through["folder"] = alongNode("project", alongWorld, nil)
		}),
	} {
		_, err := store.Rollup(ctx(), s, q)
		require.ErrorIs(t, err, store.ErrInvalidQuery, name)
	}
}
