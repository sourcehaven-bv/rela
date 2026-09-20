//go:build sqlite

package sqlitestore

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The gate decides exactness, so it is pinned directly: what is pushed, and
// — more importantly — what must decline to graphquerynaive.
func TestSimpleGraphSQL_Gate(t *testing.T) {
	scalar := store.PropPredicate{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}
	for name, tc := range map[string]struct {
		q    store.GraphQuery
		want bool
	}{
		"type only":          {store.GraphQuery{EntityType: "t"}, true},
		"scalar equality":    {store.GraphQuery{EntityType: "t", Props: []store.PropPredicate{scalar}}, true},
		"order and page":     {store.GraphQuery{EntityType: "t", OrderBy: []store.OrderSpec{{Property: "due"}}, Limit: 5, Offset: 5}, true},
		"no type":            {store.GraphQuery{}, false},
		"non-scalar equal":   {store.GraphQuery{EntityType: "t", Props: []store.PropPredicate{{Property: "status", Value: "open"}}}, false},
		"is-empty test":      {store.GraphQuery{EntityType: "t", Props: []store.PropPredicate{{Property: "status", Scalar: true}}}, false},
		"not equal":          {store.GraphQuery{EntityType: "t", Props: []store.PropPredicate{{Property: "status", Op: store.PropNotEqual, Value: "x"}}}, false},
		"odd property name":  {store.GraphQuery{EntityType: "t", Props: []store.PropPredicate{{Property: `a"b`, Value: "x", Scalar: true}}}, false},
		"odd order property": {store.GraphQuery{EntityType: "t", OrderBy: []store.OrderSpec{{Property: "a.b"}}}, false},
		"inbound predicate":  {store.GraphQuery{EntityType: "t", HasInbound: &store.RelationPredicate{}}, false},
		"outbound predicate": {store.GraphQuery{EntityType: "t", HasOutbound: &store.RelationPredicate{}}, false},
		"any branches":       {store.GraphQuery{EntityType: "t", Any: []store.GraphBranch{{}}}, false},
		"narrowing":          {store.GraphQuery{EntityType: "t", Narrowing: []store.NarrowBranch{{}}}, false},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, _, ok := simpleGraphSQL(tc.q, entityColumns, false); ok != tc.want {
				t.Errorf("pushed = %v, want %v", ok, tc.want)
			}
		})
	}
}
