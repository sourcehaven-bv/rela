//go:build sqlite

package mcp

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// The list_entities condition comparison on the sqlite SQL rendering, where
// a negated traversal and the ACL's row gate meet in one statement
// (TKT-Z099FGY).
func TestListEntities_ConditionMatchesGoPath_SQLite(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "list.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st, err := sqlitestore.New(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	assertConditionMatchesGoPath(t, newConditionFixtureOn(t, st, 24))
}
