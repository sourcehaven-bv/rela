package entitymanager

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// placeAndApply runs PlaceOrder and returns the target ids in the order the
// side reads afterwards.
func placeAndApply(
	t *testing.T, rels []entity.Relation, moved string, pos entity.OrderPosition,
) (order []string, written int, err error) {
	t.Helper()
	key := entity.RelationKey{From: "src", Type: "has-step", To: moved}
	plan, err := PlaceOrder(rels, key, pos, "_order_out")
	if err != nil {
		return nil, 0, err
	}
	after := make([]entity.Relation, len(rels))
	for i, r := range rels {
		r.Properties = maps.Clone(r.Properties)
		if v, ok := plan[r.Identity()]; ok {
			r.Properties["_order_out"] = v
		}
		after[i] = r
	}
	var ids []string
	for _, r := range SortRelations(after, "_order_out") {
		ids = append(ids, r.To)
	}
	return ids, len(plan), nil
}

func TestPlaceOrder(t *testing.T) {
	t.Parallel()
	rel := func(id string, order any) entity.Relation {
		props := map[string]any{}
		if order != nil {
			props["_order_out"] = order
		}
		return entity.Relation{From: "src", Type: "has-step", To: id, Properties: props}
	}
	valued := []entity.Relation{rel("a", 1.0), rel("b", 2.0), rel("c", 3.0), rel("d", 4.0)}
	missing := []entity.Relation{rel("a", nil), rel("b", nil), rel("c", nil), rel("d", nil)}
	tied := []entity.Relation{rel("a", 1.0), rel("b", 1.0), rel("c", 1.0), rel("d", 2.0)}
	mixed := []entity.Relation{rel("a", 1.0), rel("b", 2.0), rel("c", nil), rel("d", nil)}
	// A gap wide enough for a midpoint, but not for one that leaves the
	// collapse threshold on both sides.
	narrow := []entity.Relation{rel("a", 1.0), rel("b", 1.0+1.5e-9), rel("c", 3.0)}

	tests := []struct {
		name      string
		rels      []entity.Relation
		moved     string
		pos       entity.OrderPosition
		want      []string
		maxWrites int // 1 = only the moved edge; 0 = no-op
	}{
		{"before first", valued, "c", entity.OrderPosition{Before: "a"}, []string{"c", "a", "b", "d"}, 1},
		{"after last", valued, "a", entity.OrderPosition{After: "d"}, []string{"b", "c", "d", "a"}, 1},
		{"between", valued, "d", entity.OrderPosition{Before: "b"}, []string{"a", "d", "b", "c"}, 1},
		{"before own successor is a no-op", valued, "b", entity.OrderPosition{Before: "c"}, []string{"a", "b", "c", "d"}, 0},
		{"step up", valued, "c", entity.OrderPosition{Step: -1}, []string{"a", "c", "b", "d"}, 1},
		{"step down", valued, "b", entity.OrderPosition{Step: 1}, []string{"a", "c", "b", "d"}, 1},
		{"step up at the top is a no-op", valued, "a", entity.OrderPosition{Step: -1}, []string{"a", "b", "c", "d"}, 0},
		{"step down at the bottom is a no-op", valued, "d", entity.OrderPosition{Step: 1}, []string{"a", "b", "c", "d"}, 0},
		// Pre-existing edges with no value: the first move lands exactly
		// where it was dropped, not first (densify).
		{"all missing", missing, "d", entity.OrderPosition{Before: "b"}, []string{"a", "d", "b", "c"}, 4},
		{"all missing, to the top", missing, "c", entity.OrderPosition{Before: "a"}, []string{"c", "a", "b", "d"}, 4},
		{"tied neighbors densify", tied, "d", entity.OrderPosition{After: "a"}, []string{"a", "d", "b", "c"}, 4},
		{"after the last valued edge needs no densify", mixed, "a", entity.OrderPosition{After: "b"}, []string{"b", "a", "c", "d"}, 1},
		{"among the missing edges densifies", mixed, "a", entity.OrderPosition{After: "c"}, []string{"b", "c", "a", "d"}, 4},
		{"a near-collapsed gap densifies", narrow, "c", entity.OrderPosition{After: "a"}, []string{"a", "c", "b"}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, writes, err := placeAndApply(t, tt.rels, tt.moved, tt.pos)
			if err != nil {
				t.Fatalf("PlaceOrder: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("order = %v, want %v", got, tt.want)
			}
			if writes > tt.maxWrites {
				t.Errorf("writes = %d, want at most %d", writes, tt.maxWrites)
			}
		})
	}
}

func TestPlaceOrder_Errors(t *testing.T) {
	t.Parallel()
	rels := []entity.Relation{
		{From: "src", Type: "has-step", To: "a", Properties: map[string]any{"_order_out": 1.0}},
		{From: "src", Type: "has-step", To: "b", Properties: map[string]any{"_order_out": 2.0}},
	}
	tests := []struct {
		name  string
		moved string
		pos   entity.OrderPosition
		want  error
	}{
		{"nothing named", "a", entity.OrderPosition{}, ErrInvalidOrderPosition},
		{"two named", "a", entity.OrderPosition{Before: "b", Step: 1}, ErrInvalidOrderPosition},
		{"step of two", "a", entity.OrderPosition{Step: 2}, ErrInvalidOrderPosition},
		{"relative to itself", "a", entity.OrderPosition{After: "a"}, ErrInvalidOrderPosition},
		{"unknown sibling", "a", entity.OrderPosition{After: "zzz"}, ErrOrderRefNotSibling},
		{"moved edge missing", "zzz", entity.OrderPosition{After: "a"}, ErrRelationNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			key := entity.RelationKey{From: "src", Type: "has-step", To: tt.moved}
			if _, err := PlaceOrder(rels, key, tt.pos, "_order_out"); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestPlaceOrder_RefOnAnotherTailIsNotASibling(t *testing.T) {
	t.Parallel()
	rels := []entity.Relation{
		{From: "src", Type: "has-step", To: "a", Properties: map[string]any{"_order_out": 1.0}},
		{From: "src", FromFace: "draft", Type: "has-step", To: "b", Properties: map[string]any{"_order_out": 2.0}},
	}
	key := entity.RelationKey{From: "src", Type: "has-step", To: "a"}
	if _, err := PlaceOrder(rels, key, entity.OrderPosition{After: "b"}, "_order_out"); !errors.Is(err, ErrOrderRefNotSibling) {
		t.Errorf("err = %v, want ErrOrderRefNotSibling", err)
	}
}
