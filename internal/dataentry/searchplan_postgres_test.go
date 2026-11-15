//go:build postgres

package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
)

// The memstore comparison runs the plan's queries through graphquerynaive.
// These run them as SQL, where emptiness, `!=` on a missing value, the enum
// rank order and the native position query are pgstore's own.
func TestSearchPlan_MatchesGoPath_Postgres(t *testing.T) {
	assertSearchPlanMatchesGoPath(t, newSearchPlanAppPG)
}

func TestSearchPlan_PositionMatchesGoPath_Postgres(t *testing.T) {
	assertSearchPositionMatchesGoPath(t, newSearchPlanAppPG)
}

func newSearchPlanAppPG(t *testing.T) *App {
	t.Helper()
	_, dsn := conflictTestSchema(t)
	return newSearchPlanApp(t, appbuildtest.WithStore(openPGStore(t, dsn)))
}
