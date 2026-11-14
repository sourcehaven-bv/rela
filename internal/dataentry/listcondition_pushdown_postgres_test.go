//go:build postgres

package dataentry

import "testing"

// The list condition comparison on the postgres SQL rendering, where a
// negated traversal and the ACL's row gate meet in one statement
// (TKT-I39Y7N4).
func TestListPushdown_ConditionMatchesGoPath_Postgres(t *testing.T) {
	_, dsn := conflictTestSchema(t)
	app, _, counting := newScopePushdownAppOn(t, openPGStore(t, dsn))
	assertConditionMatchesGoPath(t, app, counting)
}
