//go:build sqlite

package visibility_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// The sqlite twin of TestScriptReader_ListEntityPage_HiddenFields: the
// guarded filter renders as sqlite SQL.
func TestScriptReader_ListEntityPage_HiddenFields_SQLite(t *testing.T) {
	t.Parallel()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{
		Path: filepath.Join(t.TempDir(), "hidden.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st, err := sqlitestore.New(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	assertHiddenFieldPages(t, newHiddenFieldFixture(t, st))
}
