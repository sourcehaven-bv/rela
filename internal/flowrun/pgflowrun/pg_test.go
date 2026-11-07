package pgflowrun_test

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

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/flowruntest"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/pgflowrun"
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
	flowruntest.RunAll(t, func(t *testing.T) flowruntest.Harness {
		t.Helper()
		pool, schema := newScopedPool(t)
		s, err := pgflowrun.New(pool)
		require.NoError(t, err)
		return flowruntest.Harness{
			Store: s,
			Second: func(t *testing.T) flowrun.Store {
				t.Helper()
				s, err := pgflowrun.New(openPool(t, schema))
				require.NoError(t, err)
				return s
			},
		}
	})
}

func TestNewRejectsNil(t *testing.T) {
	_, err := pgflowrun.New(nil)
	require.Error(t, err)
}

// TestSchemaIsolation: a run in one tenant schema is invisible in another,
// and a singleton there does not block the same subject here.
func TestSchemaIsolation(t *testing.T) {
	ctx := context.Background()
	poolA, _ := newScopedPool(t)
	poolB, _ := newScopedPool(t)
	a, err := pgflowrun.New(poolA)
	require.NoError(t, err)
	b, err := pgflowrun.New(poolB)
	require.NoError(t, err)

	t0 := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	nr := flowrun.NewRun{
		ID: "FR-1", Flow: "onboard", Version: "0123456789abcdef", Definition: []byte("d"),
		Subject: entity.Ref{ID: "EMP-1"}, Singleton: true, At: t0,
		Instance: flow.Instance{State: "open", Generation: 1, EnteredAt: t0},
	}
	_, err = a.Create(ctx, nr)
	require.NoError(t, err)
	_, err = b.Get(ctx, "FR-1")
	require.ErrorIs(t, err, flowrun.ErrNotFound)
	_, err = b.Create(ctx, nr)
	require.NoError(t, err)
}

// newScopedPool returns a pool pinned to a fresh, migrated schema, and
// the schema's name.
func newScopedPool(t *testing.T) (pool *pgxpool.Pool, schema string) {
	t.Helper()
	admin := adminConn(t)
	ctx := context.Background()

	schema = fmt.Sprintf("relaflowrun_%d_%d", os.Getpid(), schemaCounter.Add(1))
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
			t.Fatalf("%s is set, so the pgflowrun suite must run, but %s is empty", requireDBEnv, testDBEnv)
		}
		t.Skipf("%s not set; skipping pgflowrun integration tests", testDBEnv)
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
