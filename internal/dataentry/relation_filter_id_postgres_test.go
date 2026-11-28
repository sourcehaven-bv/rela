//go:build postgres

package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
)

// TestListPushdown_RelationFilterByID_Postgres runs the id relation filter
// comparison with the SQL rendering of the gated traversal.
func TestListPushdown_RelationFilterByID_Postgres(t *testing.T) {
	_, dsn := conflictTestSchema(t)
	assertRelationIDPushdown(t, withSameTitle(relationFilterAppOn(t, appbuildtest.WithStore(openPGStore(t, dsn)))))
}

// TestListPushdown_RelationFilterByIDRowGated_Postgres runs the row-gated
// parity check on postgres.
func TestListPushdown_RelationFilterByIDRowGated_Postgres(t *testing.T) {
	_, dsn := conflictTestSchema(t)
	assertRelationIDPushdownRowGated(t, withSameTitle(relationFilterAppOn(t, appbuildtest.WithStore(openPGStore(t, dsn)))))
}
