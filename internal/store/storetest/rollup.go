package storetest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunRollupTests is the conformance suite for [store.Rollup] (TKT-6I2OPG).
// Every case runs the backend's answer AND [store.RollupGeneric] on the same
// store and requires both to equal the expected result, so a native SQL
// fold cannot drift from the reference.
//
// The scenarios that matter to a backend author:
//
//   - only rows the Related query selects are folded: the caller's gate runs
//     before the fold, and a where filter is just more Props
//   - only identity-scoped edges of the asked relation, in the asked
//     direction, link a row to a source
//   - a value of the wrong shape is skipped, never coerced: a number is an
//     integral JSON number, a string is non-empty
//   - latest orders on the non-empty by string, ties going to the smallest id,
//     and a PickRequire that fails on the winner empties the value rather
//     than reading an older row
//   - Require conditions drop a row from every aggregate, count included
//
// Run via [RunAll].
func RunRollupTests(t *testing.T, f Factory) {
	t.Helper()

	// seed builds risks with evidence and tests:
	//   RSK-1 -evidenced-by-> EV-1 (geldig, score 3, 2024-01-02)
	//   RSK-1 -evidenced-by-> EV-2 (verlopen, score 7, 2024-03-01)
	//   RSK-1 -evidenced-by-> EV-3 (geldig, score "9", no datum)
	//   RSK-2 -evidenced-by-> EV-2
	//   RSK-3 has no edges
	//   TST-1 -tests-> RSK-1 (2024-05-01, geslaagd), TST-2 -tests-> RSK-1
	//   (2024-05-01, gefaald), TST-3 -tests-> RSK-1 (2024-01-01, gefaald)
	seed := func(t *testing.T, s store.Store) {
		t.Helper()
		seedGraphQueryEntities(t, s, "risk", "RSK-1", "RSK-2", "RSK-3")
		seedEntityWithProps(t, s, "evidence", "EV-1",
			map[string]any{"status": "geldig", "score": 3, "datum": "2024-01-02"})
		seedEntityWithProps(t, s, "evidence", "EV-2",
			map[string]any{"status": "verlopen", "score": 7, "datum": "2024-03-01"})
		seedEntityWithProps(t, s, "evidence", "EV-3",
			map[string]any{"status": "geldig", "score": "9", "datum": ""})
		seedEntityWithProps(t, s, "test", "TST-1", map[string]any{"datum": "2024-05-01", "resultaat": "geslaagd"})
		seedEntityWithProps(t, s, "test", "TST-2", map[string]any{"datum": "2024-05-01", "resultaat": "gefaald"})
		seedEntityWithProps(t, s, "test", "TST-3", map[string]any{"datum": "2024-01-01", "resultaat": "gefaald"})
		mustRel(t, s, "RSK-1", "evidenced-by", "EV-1")
		mustRel(t, s, "RSK-1", "evidenced-by", "EV-2")
		mustRel(t, s, "RSK-1", "evidenced-by", "EV-3")
		mustRel(t, s, "RSK-2", "evidenced-by", "EV-2")
		mustRel(t, s, "TST-1", "tests", "RSK-1")
		mustRel(t, s, "TST-2", "tests", "RSK-1")
		mustRel(t, s, "TST-3", "tests", "RSK-1")
	}
	world := store.InWorld(store.TrivialScope())
	evidence := func(props ...store.PropPredicate) store.GraphQuery {
		return store.GraphQuery{EntityType: "evidence", Faces: world, Props: props}
	}
	geldig := store.PropPredicate{Property: "status", Op: store.PropEqual, Value: "geldig", Scalar: true}
	allRisks := []string{"RSK-1", "RSK-2", "RSK-3"}

	cases := []struct {
		name string
		q    store.RollupQuery
		want map[string]store.RollupResult
	}{
		{
			name: "count_outgoing",
			q:    store.RollupQuery{Related: evidence(), Relation: "evidenced-by", Sources: allRisks},
			want: map[string]store.RollupResult{"RSK-1": {Count: 3}, "RSK-2": {Count: 1}},
		},
		{
			name: "count_where",
			q:    store.RollupQuery{Related: evidence(geldig), Relation: "evidenced-by", Sources: allRisks},
			want: map[string]store.RollupResult{"RSK-1": {Count: 2}},
		},
		{
			name: "count_incoming",
			q: store.RollupQuery{
				Related:  store.GraphQuery{EntityType: "test", Faces: world},
				Relation: "tests", SourceIsTo: true, Sources: allRisks,
			},
			want: map[string]store.RollupResult{"RSK-1": {Count: 3}},
		},
		{
			name: "wrong_direction_finds_nothing",
			q: store.RollupQuery{
				Related:  store.GraphQuery{EntityType: "test", Faces: world},
				Relation: "tests", Sources: allRisks,
			},
			want: map[string]store.RollupResult{},
		},
		{
			name: "only_asked_sources",
			q:    store.RollupQuery{Related: evidence(), Relation: "evidenced-by", Sources: []string{"RSK-2", "RSK-2"}},
			want: map[string]store.RollupResult{"RSK-2": {Count: 1}},
		},
		{
			name: "max_numeric_skips_string_number",
			q: store.RollupQuery{
				Related: evidence(geldig), Relation: "evidenced-by", Sources: allRisks,
				Func: store.RollupMax, Value: "score", Numeric: true,
			},
			want: map[string]store.RollupResult{"RSK-1": {Count: 2, Value: int64(3)}},
		},
		{
			name: "min_numeric",
			q: store.RollupQuery{
				Related: evidence(), Relation: "evidenced-by", Sources: allRisks,
				Func: store.RollupMin, Value: "score", Numeric: true,
			},
			want: map[string]store.RollupResult{
				"RSK-1": {Count: 3, Value: int64(3)}, "RSK-2": {Count: 1, Value: int64(7)},
			},
		},
		{
			name: "sum",
			q: store.RollupQuery{
				Related: evidence(), Relation: "evidenced-by", Sources: allRisks,
				Func: store.RollupSum, Value: "score",
			},
			want: map[string]store.RollupResult{
				"RSK-1": {Count: 3, Value: int64(10)}, "RSK-2": {Count: 1, Value: int64(7)},
			},
		},
		{
			name: "max_string_skips_empty",
			q: store.RollupQuery{
				Related: evidence(geldig), Relation: "evidenced-by", Sources: allRisks,
				Func: store.RollupMax, Value: "datum",
			},
			want: map[string]store.RollupResult{"RSK-1": {Count: 2, Value: "2024-01-02"}},
		},
		{
			name: "min_string",
			q: store.RollupQuery{
				Related: evidence(), Relation: "evidenced-by", Sources: allRisks,
				Func: store.RollupMin, Value: "datum",
			},
			want: map[string]store.RollupResult{
				"RSK-1": {Count: 3, Value: "2024-01-02"}, "RSK-2": {Count: 1, Value: "2024-03-01"},
			},
		},
		{
			name: "value_absent_when_no_row_has_one",
			q: store.RollupQuery{
				Related: evidence(), Relation: "evidenced-by", Sources: allRisks,
				Func: store.RollupMax, Value: "missing", Numeric: true,
			},
			want: map[string]store.RollupResult{"RSK-1": {Count: 3}, "RSK-2": {Count: 1}},
		},
		{
			name: "latest_tie_goes_to_smallest_id",
			q: store.RollupQuery{
				Related:  store.GraphQuery{EntityType: "test", Faces: world},
				Relation: "tests", SourceIsTo: true, Sources: allRisks,
				Func: store.RollupLatest, Value: "resultaat", By: "datum",
			},
			want: map[string]store.RollupResult{"RSK-1": {Count: 3, Value: "geslaagd"}},
		},
		{
			name: "latest_skips_empty_by",
			q: store.RollupQuery{
				Related: evidence(geldig), Relation: "evidenced-by", Sources: allRisks,
				Func: store.RollupLatest, Value: "status", By: "datum",
			},
			want: map[string]store.RollupResult{"RSK-1": {Count: 2, Value: "geldig"}},
		},
		{
			name: "latest_pick_number",
			q: store.RollupQuery{
				Related: evidence(), Relation: "evidenced-by", Sources: allRisks,
				Func: store.RollupLatest, Value: "score", By: "datum",
			},
			want: map[string]store.RollupResult{
				"RSK-1": {Count: 3, Value: int64(7)}, "RSK-2": {Count: 1, Value: int64(7)},
			},
		},
		{
			name: "pick_require_withholds_winner_without_fallback",
			q: store.RollupQuery{
				Related:  store.GraphQuery{EntityType: "test", Faces: world},
				Relation: "tests", SourceIsTo: true, Sources: allRisks,
				Func: store.RollupLatest, Value: "resultaat", By: "datum",
				// Visible only on failed tests: TST-1 still wins, so the
				// value is empty, never TST-2's.
				PickRequire: []store.RowCondition{{Any: []*store.GraphQuery{{Props: []store.PropPredicate{
					{Property: "resultaat", Op: store.PropEqual, Value: "gefaald", Scalar: true},
				}}}}},
			},
			want: map[string]store.RollupResult{"RSK-1": {Count: 3}},
		},
		{
			name: "require_drops_rows_from_count",
			q: store.RollupQuery{
				Related: evidence(), Relation: "evidenced-by", Sources: allRisks,
				Require: []store.RowCondition{{Any: []*store.GraphQuery{{Props: []store.PropPredicate{geldig}}}}},
			},
			want: map[string]store.RollupResult{"RSK-1": {Count: 2}},
		},
		{
			name: "require_any_nil_query_holds",
			q: store.RollupQuery{
				Related: evidence(), Relation: "evidenced-by", Sources: allRisks,
				Require: []store.RowCondition{{Any: []*store.GraphQuery{{Props: []store.PropPredicate{geldig}}, nil}}},
			},
			want: map[string]store.RollupResult{"RSK-1": {Count: 3}, "RSK-2": {Count: 1}},
		},
		{
			name: "require_empty_any_holds_for_none",
			q: store.RollupQuery{
				Related: evidence(), Relation: "evidenced-by", Sources: allRisks,
				Require: []store.RowCondition{{}},
			},
			want: map[string]store.RollupResult{},
		},
		{
			name: "require_with_relation_predicate",
			q: store.RollupQuery{
				Related: evidence(), Relation: "evidenced-by", Sources: allRisks,
				// Rows linked from RSK-2 only: EV-2.
				Require: []store.RowCondition{{Any: []*store.GraphQuery{{Related: []store.DirectedRelation{{
					Incoming: true,
					Pred:     store.RelationPredicate{OfTypes: []string{"evidenced-by"}, Endpoints: []string{"RSK-2"}},
				}}}}}},
			},
			want: map[string]store.RollupResult{"RSK-1": {Count: 1}, "RSK-2": {Count: 1}},
		},
		{
			name: "no_sources",
			q:    store.RollupQuery{Related: evidence(), Relation: "evidenced-by"},
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

	t.Run("content_scoped_edges_are_not_followed", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		face, err := entity.ParseFace("draft")
		require.NoError(t, err)
		draft := entity.New("RSK-3", "risk")
		draft.Face = face
		require.NoError(t, s.CreateEntity(ctx(), draft))
		_, err = s.CreateRelation(ctx(),
			entity.RelationKey{From: "RSK-3", FromFace: face, Type: "evidenced-by", To: "EV-1"}, nil)
		require.NoError(t, err)
		requireRollup(t, s, store.RollupQuery{
			Related: evidence(), Relation: "evidenced-by", Sources: []string{"RSK-3"},
		}, map[string]store.RollupResult{})
	})

	t.Run("world_folds_the_prime_face", func(t *testing.T) {
		s := f(t)
		seed(t, s)
		face, err := entity.ParseFace("draft")
		require.NoError(t, err)
		ev := entity.New("EV-1", "evidence")
		ev.Face = face
		ev.Properties["score"] = 100
		require.NoError(t, s.CreateEntity(ctx(), ev))
		draftFirst := store.InWorld(store.NewWorldScope(map[string]store.TypeResolution{
			"evidence": {Chain: []entity.Face{face, ""}, Fallback: store.FallbackExclude},
		}))
		q := store.RollupQuery{
			Related:  store.GraphQuery{EntityType: "evidence", Faces: draftFirst},
			Relation: "evidenced-by", Sources: allRisks,
			Func: store.RollupMax, Value: "score", Numeric: true,
		}
		requireRollup(t, s, q, map[string]store.RollupResult{
			"RSK-1": {Count: 3, Value: int64(100)}, "RSK-2": {Count: 1, Value: int64(7)},
		})
	})

	t.Run("sum_outside_int64_has_no_value", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "risk", "RSK-1", "RSK-2")
		seedEntityWithProps(t, s, "evidence", "EV-1", map[string]any{"score": int64(6e18)})
		seedEntityWithProps(t, s, "evidence", "EV-2", map[string]any{"score": int64(6e18)})
		seedEntityWithProps(t, s, "evidence", "EV-3", map[string]any{"score": int64(-6e18)})
		for _, ev := range []string{"EV-1", "EV-2"} {
			mustRel(t, s, "RSK-1", "evidenced-by", ev)
		}
		// An intermediate total outside int64 is fine when the final one
		// fits.
		for _, ev := range []string{"EV-1", "EV-2", "EV-3"} {
			mustRel(t, s, "RSK-2", "evidenced-by", ev)
		}
		requireRollup(t, s, store.RollupQuery{
			Related: evidence(), Relation: "evidenced-by", Sources: []string{"RSK-1", "RSK-2"},
			Func: store.RollupSum, Value: "score",
		}, map[string]store.RollupResult{"RSK-1": {Count: 2}, "RSK-2": {Count: 3, Value: int64(6e18)}})
	})

	// The fs store parses an unquoted YAML date into a time value; the
	// database backends hold its JSON text. Both order and return alike.
	t.Run("time_values_order_by_their_json_text", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "risk", "RSK-1")
		day := func(d int) time.Time { return time.Date(2024, time.May, d, 0, 0, 0, 0, time.UTC) }
		for id, p := range map[string]map[string]any{
			"TST-1": {"datum": day(1), "resultaat": "geslaagd", "op": day(9)},
			"TST-2": {"datum": day(3), "resultaat": "gefaald", "op": day(7)},
			"TST-3": {"datum": day(2), "resultaat": "geslaagd", "op": day(8)},
		} {
			seedEntityWithProps(t, s, "test", id, p)
			mustRel(t, s, id, "tests", "RSK-1")
		}
		tests := store.GraphQuery{EntityType: "test", Faces: world}
		requireRollup(t, s, store.RollupQuery{
			Related: tests, Relation: "tests", SourceIsTo: true, Sources: []string{"RSK-1"},
			Func: store.RollupLatest, Value: "op", By: "datum",
		}, map[string]store.RollupResult{"RSK-1": {Count: 3, Value: "2024-05-07T00:00:00Z"}})
		requireRollup(t, s, store.RollupQuery{
			Related: tests, Relation: "tests", SourceIsTo: true, Sources: []string{"RSK-1"},
			Func: store.RollupMin, Value: "datum",
		}, map[string]store.RollupResult{"RSK-1": {Count: 3, Value: "2024-05-01T00:00:00Z"}})
	})

	t.Run("malformed_values_are_skipped", func(t *testing.T) {
		s := f(t)
		seedGraphQueryEntities(t, s, "risk", "RSK-1")
		for id, v := range map[string]any{
			"EV-1": "abc", "EV-2": 1.5, "EV-3": 1e30, "EV-4": []any{"1"}, "EV-5": true, "EV-6": 2.0, "EV-7": 4,
		} {
			seedEntityWithProps(t, s, "evidence", id, map[string]any{"score": v})
			mustRel(t, s, "RSK-1", "evidenced-by", id)
		}
		for fn, want := range map[store.RollupFunc]any{
			store.RollupSum: int64(6), store.RollupMin: int64(2), store.RollupMax: int64(4),
		} {
			requireRollup(t, s, store.RollupQuery{
				Related: evidence(), Relation: "evidenced-by", Sources: []string{"RSK-1"},
				Func: fn, Value: "score", Numeric: true,
			}, map[string]store.RollupResult{"RSK-1": {Count: 7, Value: want}})
		}
	})

	t.Run("invalid_queries_are_refused", func(t *testing.T) {
		s := f(t)
		for name, q := range map[string]store.RollupQuery{
			"no_relation":   {Related: evidence(), Sources: allRisks},
			"no_world":      {Related: store.GraphQuery{EntityType: "evidence"}, Relation: "x", Sources: allRisks},
			"paged":         {Related: store.GraphQuery{EntityType: "evidence", Faces: world, Limit: 1}, Relation: "x"},
			"latest_no_by":  {Related: evidence(), Relation: "x", Func: store.RollupLatest, Value: "v"},
			"sum_no_value":  {Related: evidence(), Relation: "x", Func: store.RollupSum},
			"no_entitytype": {Related: store.GraphQuery{Faces: world}, Relation: "x"},
		} {
			_, err := store.Rollup(ctx(), s, q)
			require.ErrorIs(t, err, store.ErrInvalidQuery, name)
		}
	})
}

// requireRollup asserts the backend's answer and the generic reference both
// equal want.
func requireRollup(t *testing.T, s store.Store, q store.RollupQuery, want map[string]store.RollupResult) {
	t.Helper()
	got, err := store.Rollup(ctx(), s, q)
	require.NoError(t, err)
	require.Equal(t, want, got, "backend")
	ref, err := store.RollupGeneric(ctx(), s, q)
	require.NoError(t, err)
	require.Equal(t, want, ref, "generic reference")
}
