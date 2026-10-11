//go:build sqlite

package appbuild_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/state/statesql"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore/tokenstoretest"
)

// The sealed token store over the sqlite state store the sqlite recipe
// wires, wrapped in ValidatedKV as production wraps it.
func TestSealed_ConformanceOverSQLiteStateKV(t *testing.T) {
	tokenstoretest.RunAll(t, func(tb testing.TB) tokenstore.Store {
		tb.Helper()
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: filepath.Join(tb.TempDir(), "rela.db")})
		if err != nil {
			tb.Fatal(err)
		}
		tb.Cleanup(func() { _ = db.Close() })
		raw, err := statesql.New(db.DB())
		if err != nil {
			tb.Fatal(err)
		}
		kv, err := state.NewValidatedKV(raw)
		if err != nil {
			tb.Fatal(err)
		}
		s, err := tokenstore.NewSealed(kv, bytes.Repeat([]byte{1}, tokenstore.KeySize), "sqlite-scope")
		if err != nil {
			tb.Fatal(err)
		}
		return s
	})
}
