//go:build sqlite

package visibility_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// The sqlite twin of TestRollups_CapACL.
func TestRollups_CapACL_SQLite(t *testing.T) {
	t.Parallel()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: filepath.Join(t.TempDir(), "capacl.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st, err := sqlitestore.New(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	assertCapACL(t, st)
}
