package store_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func TestPlanElementRange_Classifies(t *testing.T) {
	gate := &store.GraphQuery{Props: []store.PropPredicate{{Property: "a", Op: store.PropEqual, Value: "x"}}}
	scope := store.ElementScope{
		Entities: map[string]store.ElementTypeScope{
			"zeta":   {AllowAll: true, Visibility: &store.FieldVisibility{}},
			"alpha":  {Query: gate},
			"denied": {},
			"mid": {AllowAll: true, Faces: []entity.Face{"draft"}, Visibility: &store.FieldVisibility{
				Hidden: []store.HiddenField{{Property: "p"}},
			}},
		},
		Relations: map[string]store.RelationElementScope{
			"owns":  {Visibility: &store.FieldVisibility{}},
			"links": {},
		},
	}

	plan, err := store.PlanElementRange(scope, store.ElementRange{Unbounded: true})
	require.NoError(t, err)
	require.Len(t, plan.Types, 3)
	require.Equal(t, "alpha", plan.Types[0].Type)
	require.Equal(t, store.FoldPending, plan.Types[0].Fold)
	require.Equal(t, "alpha", plan.Types[0].Gate.EntityType)
	require.Equal(t, store.AllFaces(), plan.Types[0].Gate.Faces)
	require.Empty(t, gate.EntityType, "the gate template must not change")
	require.Equal(t, [2]string{"alpha/", "alpha0"}, [2]string{plan.Types[0].Lo, plan.Types[0].Hi})
	require.Equal(t, "mid", plan.Types[1].Type)
	require.Equal(t, store.FoldPerRow, plan.Types[1].Fold)
	require.True(t, plan.Types[1].Admits("draft"))
	require.False(t, plan.Types[1].Admits(""))
	require.Equal(t, store.FoldStored, plan.Types[2].Fold)
	require.Nil(t, plan.Types[2].Gate)
	require.Empty(t, plan.Relations, "an entity range plans no relation types")

	plan, err = store.PlanElementRange(scope, store.ElementRange{Relations: true, Unbounded: true})
	require.NoError(t, err)
	require.Len(t, plan.Types, 3, "a relation range gates endpoints of every admitted type")
	require.Equal(t, []string{"links", "owns"}, plan.RelationTypes())
	require.Equal(t, []string{"owns"}, plan.FoldTypes())
}

func TestPlanElementRange_TypeWindow(t *testing.T) {
	scope := store.ElementScope{Entities: map[string]store.ElementTypeScope{
		"a":   {AllowAll: true},
		"a-b": {AllowAll: true},
		"b":   {AllowAll: true},
	}}
	cases := []struct {
		name string
		r    store.ElementRange
		want map[string][2]string
	}{
		{"unbounded", store.ElementRange{Unbounded: true},
			map[string][2]string{"a": {"a/", "a0"}, "a-b": {"a-b/", "a-b0"}, "b": {"b/", "b0"}}},
		// "a-b/..." sorts below "a/x": '-' is 0x2D, '/' is 0x2F.
		{"clipped", store.ElementRange{Lo: "a/x", Hi: "b/m"},
			map[string][2]string{"a": {"a/x", "a0"}, "b": {"b/", "b/m"}}},
		{"between types", store.ElementRange{Lo: "a-a", Hi: "a/"},
			map[string][2]string{"a-b": {"a-b/", "a-b0"}}},
		{"inside one type", store.ElementRange{Lo: "a-b/1", Hi: "a-b/2"},
			map[string][2]string{"a-b": {"a-b/1", "a-b/2"}}},
		{"empty", store.ElementRange{}, map[string][2]string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := store.PlanElementRange(scope, tc.r)
			require.NoError(t, err)
			got := map[string][2]string{}
			for _, tp := range plan.Types {
				got[tp.Type] = [2]string{tp.Lo, tp.Hi}
			}
			require.Equal(t, tc.want, got)
			require.Equal(t, len(tc.want) == 0, plan.Empty())
		})
	}
}

func TestTypePlan_Probes(t *testing.T) {
	when := &store.GraphQuery{Props: []store.PropPredicate{{Property: "show", Op: store.PropEqual, Value: "yes"}}}
	scope := store.ElementScope{Entities: map[string]store.ElementTypeScope{"doc": {
		Query: &store.GraphQuery{},
		Visibility: &store.FieldVisibility{Hidden: []store.HiddenField{
			{Property: "a", Visible: store.RowCondition{Any: []*store.GraphQuery{nil, when}}},
			{Property: "b"},
		}},
	}}}
	plan, err := store.PlanElementRange(scope, store.ElementRange{Unbounded: true})
	require.NoError(t, err)
	probes := plan.Types[0].Probes()
	require.Len(t, probes, 2, "the gate and the one non-nil condition")
	for _, p := range probes {
		require.Equal(t, "doc", p.EntityType)
		require.Equal(t, store.AllFaces(), p.Faces)
	}
	require.Equal(t, when.Props, probes[1].Props)
	require.Empty(t, when.EntityType, "the condition template must not change")
}

func TestPlanElementRange_EmptyBoundedRange(t *testing.T) {
	scope := store.ElementScope{
		Entities:  map[string]store.ElementTypeScope{"a": {AllowAll: true}},
		Relations: map[string]store.RelationElementScope{"r": {}},
	}
	plan, err := store.PlanElementRange(scope, store.ElementRange{Relations: true, Lo: "x", Hi: "x"})
	require.NoError(t, err)
	require.True(t, plan.Empty())
	plan, err = store.PlanElementRange(scope, store.ElementRange{Relations: true, Lo: "x", Unbounded: true})
	require.NoError(t, err)
	require.False(t, plan.Empty())
}

func TestRangeSumFromAggregates(t *testing.T) {
	sum, err := store.RangeSumFromAggregates(3, 1, [4]int64{1, 2, 3, 4})
	require.NoError(t, err)
	require.Equal(t, store.RangeSum{Count: 3, Pending: 1, LimbSums: [4]uint64{1, 2, 3, 4}}, sum)
	for _, tc := range []struct {
		count, pending int64
		limbs          [4]int64
	}{
		{-1, 0, [4]int64{}},
		{1, 2, [4]int64{}},
		{1, -1, [4]int64{}},
		{1, 0, [4]int64{0, 0, -1, 0}},
	} {
		_, err := store.RangeSumFromAggregates(tc.count, tc.pending, tc.limbs)
		require.Error(t, err, "%+v", tc)
	}
}

func TestRangeEntry_SetRowKey(t *testing.T) {
	var e store.RangeEntry
	e.SetRowKey(false, "doc", "D1", "draft", "")
	require.Equal(t, "doc", e.Type)
	require.Equal(t, entity.Ref{ID: "D1", Face: "draft"}, e.Ref)
	var r store.RangeEntry
	r.SetRowKey(true, "A", "draft", "links", "B")
	require.Equal(t, entity.RelationKey{From: "A", FromFace: "draft", Type: "links", To: "B"}, r.Relation)
}

func TestValidateRangeList(t *testing.T) {
	require.NoError(t, store.ValidateRangeList(store.RangeList{Limit: 1}))
	require.NoError(t, store.ValidateRangeList(store.RangeList{Limit: store.MaxRangeList}))
	require.ErrorIs(t, store.ValidateRangeList(store.RangeList{}), store.ErrInvalidQuery)
	require.ErrorIs(t, store.ValidateRangeList(store.RangeList{Limit: store.MaxRangeList + 1}), store.ErrInvalidQuery)
}
