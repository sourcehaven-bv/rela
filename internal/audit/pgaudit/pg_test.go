package pgaudit_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/audit/pgaudit"
	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit"
	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit/sqlaudittest"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec/pgexec"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// testDBEnv and requireDBEnv mirror the pgstore suite's gate: a clean skip
// by default, a failure when the environment promised a database.
const (
	testDBEnv    = "RELA_TEST_DATABASE_URL"
	requireDBEnv = "RELA_TEST_DATABASE_REQUIRED"
)

var (
	adminPoolOnce sync.Once
	adminPool     *pgxpool.Pool
	errAdminPool  error
	schemaCounter atomic.Int64
)

// TestConformance runs the shared contract against PostgreSQL, each test in
// its own schema-pinned pool. Second opens an independent pool on the same
// schema, standing in for a second rela-server process.
func TestConformance(t *testing.T) {
	sqlaudittest.RunAll(t, func(t *testing.T) sqlaudittest.Harness {
		t.Helper()
		pool, schema := newScopedPool(t)
		log, err := pgaudit.New(pool)
		require.NoError(t, err)
		raw, err := pgexec.New(pool)
		require.NoError(t, err)
		return sqlaudittest.Harness{
			Log: log,
			Raw: raw,
			Second: func(t *testing.T) *sqlaudit.Log {
				t.Helper()
				l, err := pgaudit.New(openPool(t, schema))
				require.NoError(t, err)
				return l
			},
		}
	})
}

func TestNewRejectsNil(t *testing.T) {
	_, err := pgaudit.New(nil)
	require.Error(t, err)
}

// TestSchemaIsolation: each tenant schema has its own chain.
func TestSchemaIsolation(t *testing.T) {
	ctx := context.Background()
	poolA, _ := newScopedPool(t)
	poolB, _ := newScopedPool(t)
	a, err := pgaudit.New(poolA)
	require.NoError(t, err)
	b, err := pgaudit.New(poolB)
	require.NoError(t, err)

	r := audit.Record{Time: time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC), Op: audit.OpCreateEntity}
	require.NoError(t, a.Append(ctx, r))
	require.NoError(t, a.Append(ctx, r))
	require.NoError(t, b.Append(ctx, r))

	va, err := a.Verify(ctx)
	require.NoError(t, err)
	vb, err := b.Verify(ctx)
	require.NoError(t, err)
	require.Nil(t, va.Break)
	require.Nil(t, vb.Break)
	require.Equal(t, int64(2), va.Rows)
	require.Equal(t, int64(1), vb.Rows)
}

// newScopedPool returns a pool pinned to a fresh, migrated schema, and
// the schema's name.
func newScopedPool(t *testing.T) (pool *pgxpool.Pool, schema string) {
	t.Helper()
	admin := adminConn(t)
	ctx := context.Background()

	schema = fmt.Sprintf("relaaudit_%d_%d", os.Getpid(), schemaCounter.Add(1))
	_, err := admin.Exec(ctx, "CREATE SCHEMA "+quoteIdent(schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoteIdent(schema)+" CASCADE")
	})
	pool = openPool(t, schema)
	require.NoError(t, pgstore.Migrate(ctx, pool))
	return pool, schema
}

// openPool opens an independent pool whose search_path is schema, set on
// the parsed config so a search_path in the DSN cannot override it.
func openPool(t *testing.T, schema string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(os.Getenv(testDBEnv))
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	cfg.MaxConns = 4
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func adminConn(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(testDBEnv)
	if dsn == "" {
		if v := os.Getenv(requireDBEnv); v != "" && v != "0" && !strings.EqualFold(v, "false") {
			t.Fatalf("%s is set, so the pgaudit suite must run, but %s is empty", requireDBEnv, testDBEnv)
		}
		t.Skipf("%s not set; skipping pgaudit integration tests", testDBEnv)
	}
	adminPoolOnce.Do(func() {
		adminPool, errAdminPool = pgxpool.New(context.Background(), dsn)
	})
	require.NoError(t, errAdminPool)
	return adminPool
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
