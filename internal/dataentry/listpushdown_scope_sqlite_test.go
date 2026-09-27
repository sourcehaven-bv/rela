//go:build sqlite

package dataentry

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// sqliteStore opens an empty sqlitestore in a temp dir.
func sqliteStore(t *testing.T) store.Store {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{
		Path: filepath.Join(t.TempDir(), "pushdown.db"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	st, err := sqlitestore.New(db)
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// The postgres twin of this comparison, run on the sqlite SQL rendering
// (TKT-B51CYD).
func TestListPushdown_ScopeMatchesGoPath_SQLite(t *testing.T) {
	app, d, counting := newScopePushdownAppOn(t, sqliteStore(t))
	assertScopeMatchesGoPath(t, app, d, counting)
}

// The budgets hold on sqlite too. On the SQL path each counted store call is
// one statement (GraphCount's two excepted, which a list page does not use),
// so an equal count at 10 and 50 rows is an equal statement count.
func TestQueryBudget_ListPageIsSizeIndependent_SQLite(t *testing.T) {
	small, large, detail := readsForOn(t, sqliteStore, listPageOp)
	assertBudget(t, "list page", small, large, listPageBudget, detail)
}

func TestQueryBudget_TraversalScopeIsPushedDown_SQLite(t *testing.T) {
	assertScopePushedDown(t, sqliteStore)
}
