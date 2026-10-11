package pgstore

import (
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// D7: an unknown property operator fails the build with ErrInvalidQuery
// instead of rendering an equality. The shape check refuses it first; this
// pins the builder's own arm.
func TestBuildGraphQuerySQL_UnknownOperator(t *testing.T) {
	p := store.PropPredicate{Property: "status", Op: store.PropOp(99), Value: "x"}
	for _, q := range []store.GraphQuery{
		{EntityType: "ticket", Props: []store.PropPredicate{p}},
		{EntityType: "ticket", Narrowing: []store.NarrowBranch{{Props: []store.PropPredicate{p}}}},
	} {
		if _, _, err := buildGraphQuerySQL(q, false); !errors.Is(err, store.ErrInvalidQuery) {
			t.Errorf("rows: err = %v, want ErrInvalidQuery", err)
		}
		if _, _, err := buildMatchingFacesSQL(q, []string{"T-1"}); !errors.Is(err, store.ErrInvalidQuery) {
			t.Errorf("matching faces: err = %v, want ErrInvalidQuery", err)
		}
		if _, _, err := buildGraphPositionSQL(q, "T-1"); !errors.Is(err, store.ErrInvalidQuery) {
			t.Errorf("position: err = %v, want ErrInvalidQuery", err)
		}
	}
}
