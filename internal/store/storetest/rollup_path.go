package storetest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// runRollupPathTests is the conformance suite for a roll-up over a path of
// relations (TKT-LTSNHD), run from [RunRollupTests]. The scenarios that
// matter to a backend author:
//
//   - a related row reached from one source by several paths counts once,
//     and folds once into a sum
//   - a step's Through gates the entities it reaches: one it does not select
//     breaks every path through it, and a world picks the face it judges
//   - each step walks its own direction
//   - only identity-scoped edges are followed on a step
func runRollupPathTests(t *testing.T, f Factory) {
	t.Helper()

	// seed builds risks mapped to controls, with evidence and tests on the
	// controls and reviewers on the evidence:
	//   RSK-1 -maps-> CTL-1, CTL-2     RSK-2 -maps-> CTL-2, CTL-3
	//   CTL-1 -evidenced-by-> EV-1, EV-2
	//   CTL-2 -evidenced-by-> EV-2, EV-3 (EV-2 is reached twice from RSK-1)
	//   CTL-3 -evidenced-by-> EV-4
	//   TST-1 -tests-> CTL-1           EV-1, EV-2 -reviewed-by-> PERS-1
	//   CTL-2 is closed; the others are open.
	seed := func(t *testing.T, s store.Store) {
		t.Helper()
		seedGraphQueryEntities(t, s, "risk", "RSK-1", "RSK-2", "RSK-3")
		seedGraphQueryEntities(t, s, "person", "PERS-1")
		seedEntityWithProps(t, s, "control", "CTL-1", map[string]any{"open": "ja"})
		seedEntityWithProps(t, s, "control", "CTL-2", map[string]any{"open": "nee"})
		seedEntityWithProps(t, s, "control", "CTL-3", map[string]any{"open": "ja"})
		for id, p := range map[string]map[string]any{
			"EV-1": {"score": 1, "datum": "2024-01-01", "status": "geldig"},
			"EV-2": {"score": 10, "datum": "2024-03-01", "status": "verlopen"},
			"EV-3": {"score": 100, "datum": "2024-02-01", "status": "geldig"},
			"EV-4": {"score": 1000, "datum": "2024-04-01", "status": "geldig"},
		} {
			seedEntityWithProps(t, s, "evidence", id, p)
		}
		seedEntityWithProps(t, s, "test", "TST-1", map[string]any{"datum": "2024-05-01"})
		mustRel(t, s, "RSK-1", "maps", "CTL-1")
		mustRel(t, s, "RSK-1", "maps", "CTL-2")
		mustRel(t, s, "RSK-2", "maps", "CTL-2")
		mustRel(t, s, "RSK-2", "maps", "CTL-3")
		mustRel(t, s, "CTL-1", "evidenced-by", "EV-1")
		mustRel(t, s, "CTL-1", "evidenced-by", "EV-2")
		mustRel(t, s, "CTL-2", "evidenced-by", "EV-2")
		mustRel(t, s, "CTL-2", "evidenced-by", "EV-3")
		mustRel(t, s, "CTL-3", "evidenced-by", "EV-4")
		mustRel(t, s, "TST-1", "tests", "CTL-1")
		mustRel(t, s, "EV-1", "reviewed-by", "PERS-1")
		mustRel(t, s, "EV-2", "reviewed-by", "PERS-1")
	}
	world := store.InWorld(store.TrivialScope())
	of := func(typ string, props ...store.PropPredicate) store.GraphQuery {
		return store.GraphQuery{EntityType: typ, Faces: world, Props: props}
	}
	open := store.PropPredicate{Property: "open", Op: store.PropEqual, Value: "ja", Scalar: true}
	geldig := store.PropPredicate{Property: "status", Op: store.PropEqual, Value: "geldig", Scalar: true}
	viaControls := []store.RollupStep{{Relation: "maps", Through: of("control")}}
	viaOpenControls := []store.RollupStep{{Relation: "maps", Through: of("control", open)}}
	allRisks := []string{"RSK-1", "RSK-2", "RSK-3"}
	evidence := func(q store.RollupQuery) store.RollupQuery {
		q.Relation, q.Sources = "evidenced-by", allRisks
		if q.Related.EntityType == "" {
			q.Related = of("evidence")
		}
		return q
	}

	cases := []struct {
		name string
		q    store.RollupQuery
		want map[string]store.RollupResult
	}{
		{
			name: "count_counts_a_row_reached_twice_once",
			q:    evidence(store.RollupQuery{Path: viaControls}),
			want: map[string]store.RollupResult{"RSK-1": {Count: 3}, "RSK-2": {Count: 3}},
		},
		{
			name: "sum_folds_a_row_reached_twice_once",
			q:    evidence(store.RollupQuery{Path: viaControls, Func: store.RollupSum, Value: "score"}),
			want: map[string]store.RollupResult{
				"RSK-1": {Count: 3, Value: int64(111)}, "RSK-2": {Count: 3, Value: int64(1110)},
			},
		},
		{
			name: "where_filters_the_far_end",
			q:    evidence(store.RollupQuery{Path: viaControls, Related: of("evidence", geldig)}),
			want: map[string]store.RollupResult{"RSK-1": {Count: 2}, "RSK-2": {Count: 2}},
		},
		{
			name: "gated_out_step_breaks_the_path",
			q:    evidence(store.RollupQuery{Path: viaOpenControls}),
			// CTL-2 is closed: RSK-1 keeps EV-1 and EV-2 through CTL-1,
			// RSK-2 keeps EV-4 through CTL-3. EV-3 is reached only
			// through CTL-2.
			want: map[string]store.RollupResult{"RSK-1": {Count: 2}, "RSK-2": {Count: 1}},
		},
		{
			name: "latest_over_a_path",
			q: evidence(store.RollupQuery{
				Path: viaOpenControls, Func: store.RollupLatest, Value: "score", By: "datum",
			}),
			want: map[string]store.RollupResult{
				"RSK-1": {Count: 2, Value: int64(10)}, "RSK-2": {Count: 1, Value: int64(1000)},
			},
		},
		{
			name: "min_over_a_path",
			q:    evidence(store.RollupQuery{Path: viaControls, Func: store.RollupMin, Value: "datum"}),
			want: map[string]store.RollupResult{
				"RSK-1": {Count: 3, Value: "2024-01-01"}, "RSK-2": {Count: 3, Value: "2024-02-01"},
			},
		},
		{
			name: "incoming_steps",
			// From evidence back to the risks: EV-2 reaches RSK-1 through
			// CTL-1 and CTL-2, and RSK-2 through CTL-2.
			q: store.RollupQuery{
				Path:     []store.RollupStep{{Relation: "evidenced-by", SourceIsTo: true, Through: of("control")}},
				Relation: "maps", SourceIsTo: true, Related: of("risk"),
				Sources: []string{"EV-1", "EV-2", "EV-4"},
			},
			want: map[string]store.RollupResult{
				"EV-1": {Count: 1}, "EV-2": {Count: 2}, "EV-4": {Count: 1},
			},
		},
		{
			name: "mixed_directions",
			q: store.RollupQuery{
				Path:     viaControls,
				Relation: "tests", SourceIsTo: true, Related: of("test"), Sources: allRisks,
			},
			want: map[string]store.RollupResult{"RSK-1": {Count: 1}},
		},
		{
			name: "three_relations",
			q: store.RollupQuery{
				Path: []store.RollupStep{
					{Relation: "maps", Through: of("control")},
					{Relation: "evidenced-by", Through: of("evidence")},
				},
				Relation: "reviewed-by", Related: of("person"), Sources: allRisks,
			},
			// RSK-2 reaches EV-2 through CTL-2, so PERS-1 too.
			want: map[string]store.RollupResult{"RSK-1": {Count: 1}, "RSK-2": {Count: 1}},
		},
		{
			name: "three_relations_gated_in_the_middle",
			q: store.RollupQuery{
				Path: []store.RollupStep{
					{Relation: "maps", Through: of("control", open)},
					{Relation: "evidenced-by", Through: of("evidence", geldig)},
				},
				Relation: "reviewed-by", Related: of("person"), Sources: allRisks,
			},
			// Only EV-1 (through CTL-1) is geldig and reviewed.
			want: map[string]store.RollupResult{"RSK-1": {Count: 1}},
		},
		{
			name: "wrong_type_on_a_step_finds_nothing",
			q:    evidence(store.RollupQuery{Path: []store.RollupStep{{Relation: "maps", Through: of("evidence")}}}),
			want: map[string]store.RollupResult{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := f(t)
			seed(t, s)
			requireRollup(t, s, tc.q, tc.want)
		})
	}

	// A world picks the face a step judges: under draft-first, CTL-1's
	// draft is closed, so the path through CTL-1 breaks although its prime
	// is open.
	t.Run("world_picks_the_step_face", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		face, err := entity.ParseFace("draft")
		require.NoError(t, err)
		draft := entity.New("CTL-1", "control")
		draft.Face = face
		draft.Properties["open"] = "nee"
		require.NoError(t, s.CreateEntity(ctx(), draft))
		draftFirst := store.InWorld(store.NewWorldScope(map[string]store.TypeResolution{
			"control": {Chain: []entity.Face{face, ""}, Fallback: store.FallbackExclude},
		}))
		through := store.GraphQuery{EntityType: "control", Faces: draftFirst, Props: []store.PropPredicate{open}}
		requireRollup(t, s, evidence(store.RollupQuery{Path: []store.RollupStep{{Relation: "maps", Through: through}}}),
			map[string]store.RollupResult{"RSK-2": {Count: 1}})
	})

	// The shape a read grant held through a relation compiles to, on a
	// step and on the related rows at once, so a backend that renders both
	// in one statement must keep their parts apart.
	t.Run("step_and_related_gated_by_relation_predicates", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		mustRel(t, s, "PERS-1", "shared-with", "CTL-3")
		mustRel(t, s, "PERS-1", "shared-with", "CTL-2")
		mustRel(t, s, "PERS-1", "shared-with", "EV-4")
		mustRel(t, s, "PERS-1", "shared-with", "EV-3")
		shared := func(typ string) store.GraphQuery {
			q := of(typ)
			q.HasInbound = &store.RelationPredicate{Endpoints: []string{"PERS-1"}, OfTypes: []string{"shared-with"}}
			return q
		}
		requireRollup(t, s, evidence(store.RollupQuery{
			Path:    []store.RollupStep{{Relation: "maps", Through: shared("control")}},
			Related: shared("evidence"),
		}), map[string]store.RollupResult{"RSK-1": {Count: 1}, "RSK-2": {Count: 2}})
	})

	t.Run("content_scoped_edges_are_not_followed_on_a_step", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		face, err := entity.ParseFace("draft")
		require.NoError(t, err)
		draft := entity.New("RSK-3", "risk")
		draft.Face = face
		require.NoError(t, s.CreateEntity(ctx(), draft))
		_, err = s.CreateRelation(ctx(), entity.RelationKey{From: "RSK-3", FromFace: face, Type: "maps", To: "CTL-3"}, nil)
		require.NoError(t, err)
		q := evidence(store.RollupQuery{Path: viaControls})
		q.Sources = []string{"RSK-3"}
		requireRollup(t, s, q, map[string]store.RollupResult{})
	})

	t.Run("invalid_steps_are_refused", func(t *testing.T) {
		s := f(t)
		for name, step := range map[string]store.RollupStep{
			"no_relation":   {Through: of("control")},
			"no_world":      {Relation: "maps", Through: store.GraphQuery{EntityType: "control"}},
			"no_entitytype": {Relation: "maps", Through: store.GraphQuery{Faces: world}},
			"paged":         {Relation: "maps", Through: store.GraphQuery{EntityType: "control", Faces: world, Limit: 1}},
		} {
			_, err := store.Rollup(ctx(), s, evidence(store.RollupQuery{Path: []store.RollupStep{step}}))
			require.ErrorIs(t, err, store.ErrInvalidQuery, name)
		}
	})
}
