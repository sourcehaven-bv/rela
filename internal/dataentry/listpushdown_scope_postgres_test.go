//go:build postgres

package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The memstore comparison runs Related through graphquerynaive. This runs the
// same comparison with the SQL rendering, where a gated traversal's endpoint
// row gate and the ACL's HasInbound meet in one statement.
func TestListPushdown_ScopeMatchesGoPath_Postgres(t *testing.T) {
	_, dsn := conflictTestSchema(t)
	app, d, counting := newScopePushdownAppOn(t, openPGStore(t, dsn))
	assertScopeMatchesGoPath(t, app, d, counting)
}

// The page-scoped list reads its rows by id (MatchingFaces, then a header
// read per face). On postgres both are SQL, so this pins that the id read
// stays scale-free there too (TKT-F2EJQY8).
func TestQueryBudget_PageScopedListIsSizeIndependent_Postgres(t *testing.T) {
	b, detail := readsForOn(t, func(t *testing.T) store.Store {
		t.Helper()
		_, dsn := conflictTestSchema(t)
		return openPGStore(t, dsn)
	}, pageScopedListOp)
	assertBudget(t, "page-scoped list (postgres)", b, pageScopedListBudget, detail)
}
