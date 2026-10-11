package graphquerynaive

import (
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// D7: an operator this package does not know is refused, never read as an
// equality. The shape check refuses it first; this pins the arm itself.
func TestMatchesProp_UnknownOperator(t *testing.T) {
	e := &entity.Entity{ID: "T-1", Properties: map[string]any{"status": "x"}}
	ok, err := matchesProp(e, store.PropPredicate{Property: "status", Op: store.PropOp(99), Value: "x"})
	if ok || !errors.Is(err, store.ErrInvalidQuery) {
		t.Fatalf("got %v, %v; want ErrInvalidQuery", ok, err)
	}
}
