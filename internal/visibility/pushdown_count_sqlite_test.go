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
	st, err := sqlitestore.New(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	assertCountMatchesList(t, st)
}

// The sqlite twin of TestHolderLookup_GatedLikeList: the holder predicate,
// the face allowlist and the paging render as SQL here.
func TestHolderLookup_GatedLikeList_SQLite(t *testing.T) {
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{
		Path: filepath.Join(t.TempDir(), "holders.db"),
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
	assertHoldersLikeList(t, st)
}
