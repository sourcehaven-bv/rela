//go:build postgres

package mcp

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// The list_entities condition comparison on the postgres SQL rendering,
// where a negated traversal and the ACL's row gate meet in one statement
// (TKT-Z099FGY). It runs in a private, migrated schema.
func TestListEntities_ConditionMatchesGoPath_Postgres(t *testing.T) {
	dsn := backendtest.DSN(t)
	pool, closer, err := pgstore.NewPool(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err = pgstore.Migrate(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	st, _, err := pgstore.Open(t.Context(), pool, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = st.Close()
		_ = closer.Close()
	})
	assertConditionMatchesGoPath(t, newConditionFixtureOn(t, st, 24))
}
