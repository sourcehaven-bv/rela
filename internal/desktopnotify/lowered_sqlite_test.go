//go:build sqlite

package desktopnotify

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// The lowered conditions on the sqlite SQL rendering, the desktop's own
// backend.
func TestEvaluate_LoweredMatchesGo_SQLite(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "desktop.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st, err := sqlitestore.New(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	seedMixed(t, st)
	assertLoweredMatchesGo(t, st)
}
