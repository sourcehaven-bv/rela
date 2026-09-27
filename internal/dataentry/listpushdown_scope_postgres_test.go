//go:build postgres

package dataentry

import (
	"context"
	"testing"
)

// The memstore comparison runs Related through graphquerynaive. This runs the
// same comparison with the SQL rendering, where a gated traversal's endpoint
// row gate and the ACL's HasInbound meet in one statement.
func TestListPushdown_ScopeMatchesGoPath_Postgres(t *testing.T) {
	_, dsn := conflictTestSchema(t)
	app, d, counting := newScopePushdownAppOn(t, openPGStore(t, context.Background(), dsn))
	assertScopeMatchesGoPath(t, app, d, counting)
}
