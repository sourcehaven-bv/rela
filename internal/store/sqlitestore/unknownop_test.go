package sqlitestore

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// D7: an unknown property operator marks the statement unsafe, so the store
// answers through graphquerynaive, which refuses it with ErrInvalidQuery
// (pinned end to end by storetest). It is never rendered as an equality.
func TestPropCondOn_UnknownOperator(t *testing.T) {
	b := &sqlBuilder{}
	propCondOn(b, "e", store.PropPredicate{Property: "status", Op: store.PropOp(99), Value: "x"})
	if !b.unsafe {
		t.Fatal("unknown operator rendered as a runnable condition")
	}
}
