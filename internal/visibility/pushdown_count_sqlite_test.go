//go:build sqlite

package visibility

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// The sqlite twin of TestCountPushdown_EqualsListPushdown: the count and the
// list render as different SQL, so parity is checked on the backend itself.
func TestCountPushdown_EqualsListPushdown_SQLite(t *testing.T) {
	t.Parallel()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{
		Path: filepath.Join(t.TempDir(), "count.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st, err := sqlitestore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	assertCountMatchesList(t, st)
}
