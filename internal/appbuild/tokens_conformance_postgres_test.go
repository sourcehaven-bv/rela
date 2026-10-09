//go:build postgres

package appbuild_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore/tokenstoretest"
)

// The sealed token store over pgstore's StateKV, through a schema-pinned
// DSN: the bare DSN resolves to public, the one case that always worked.
// Skips without RELA_TEST_DATABASE_URL.
func TestSealed_ConformanceOverPostgresStateKV(t *testing.T) {
	tokenstoretest.RunAll(t, func(tb testing.TB) tokenstore.Store {
		tb.Helper()
		ctx := context.Background()
		pool, err := pgxpool.New(ctx, backendtest.DSN(tb))
		if err != nil {
			tb.Fatal(err)
		}
		tb.Cleanup(pool.Close)
		if err = pgstore.Migrate(ctx, pool); err != nil {
			tb.Fatal(err)
		}
		raw, err := pgstore.NewStateKV(pool)
		if err != nil {
			tb.Fatal(err)
		}
		kv, err := state.NewValidatedKV(raw)
		if err != nil {
			tb.Fatal(err)
		}
		s, err := tokenstore.NewSealed(kv, bytes.Repeat([]byte{1}, tokenstore.KeySize), "pg:conformance")
		if err != nil {
			tb.Fatal(err)
		}
		return s
	})
}
