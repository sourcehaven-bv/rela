//go:build sqlite

package dataentry

import "testing"

// The list condition comparison on the sqlite SQL rendering (TKT-I39Y7N4).
func TestListPushdown_ConditionMatchesGoPath_SQLite(t *testing.T) {
	app, _, counting := newScopePushdownAppOn(t, sqliteStore(t))
	assertConditionMatchesGoPath(t, app, counting)
}
