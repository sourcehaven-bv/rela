package pgpiles_test

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

	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/piles/pgpiles"
	"github.com/Sourcehaven-BV/rela/internal/piles/pilestest"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// testDBEnv and requireDBEnv mirror the pgstore suite's gate: skip by
// default, but fail when the environment has promised a database, because a
// skip and a pass look the same in `go test` exit codes.
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

// TestConformance runs the shared backend contract against PostgreSQL, on a
// schema-pinned pool as production uses for a tenant.
func TestConformance(t *testing.T) {
	pilestest.RunAll(t, func(t *testing.T) piles.Store {
		t.Helper()
		s, err := pgpiles.New(newScopedPool(t))
		require.NoError(t, err)
		return s
	})
}

func TestNewRejectsNilHandle(t *testing.T) {
	_, err := pgpiles.New(nil)
	require.Error(t, err)
}

// TestSchemaIsolation proves piles are scoped per schema, which is what makes
// schema-per-tenant safe.
func TestSchemaIsolation(t *testing.T) {
	ctx := context.Background()
	a, err := pgpiles.New(newScopedPool(t))
	require.NoError(t, err)
	b, err := pgpiles.New(newScopedPool(t))
	require.NoError(t, err)

	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, a.CreatePile(ctx, piles.Pile{
		ID: "PIL-ISO00001", Owner: "alice", Name: "Inbox", Icon: piles.DefaultIcon, Created: now, Updated: now,
	}, nil, piles.MaxPiles, piles.MaxItems))

	got, err := b.ListPiles(ctx, "alice")
	require.NoError(t, err)
	require.Empty(t, got, "a pile in one schema must be invisible in another")
}

// TestCrossProcessVisibility is why this backend exists: two pools, standing
// in for two rela-server processes, see each other's piles.
func TestCrossProcessVisibility(t *testing.T) {
	ctx := context.Background()
	pool, dsn := newScopedPoolWithDSN(t)
	writer, err := pgpiles.New(pool)
	require.NoError(t, err)

	readerPool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(readerPool.Close)
	reader, err := pgpiles.New(readerPool)
	require.NoError(t, err)

	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, writer.CreatePile(ctx, piles.Pile{
		ID: "PIL-XPROC001", Owner: "alice", Name: "Inbox", Icon: piles.DefaultIcon, Created: now, Updated: now,
	}, nil, piles.MaxPiles, piles.MaxItems))

	got, err := reader.ListPiles(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, got, 1, "node B must see a pile created through node A")
}

func newScopedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, _ := newScopedPoolWithDSN(t)
	return pool
}

// newScopedPoolWithDSN returns a pool pinned to a fresh, migrated schema and
// a DSN pinned to the same schema.
func newScopedPoolWithDSN(t *testing.T) (pool *pgxpool.Pool, scopedDSN string) {
	t.Helper()
	admin := adminConn(t)
	ctx := context.Background()

	schema := fmt.Sprintf("relapiles_%d_%d", os.Getpid(), schemaCounter.Add(1))
	_, err := admin.Exec(ctx, "CREATE SCHEMA "+quoteIdent(schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoteIdent(schema)+" CASCADE")
	})

	cfg, err := pgxpool.ParseConfig(os.Getenv(testDBEnv))
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	// The concurrency tests need a few connections; the suite builds a pool
	// per subtest, so keep each one small.
	cfg.MaxConns = 4
	cfg.MinConns = 0

	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// The piles tables are part of the pgstore ladder, the same call the
	// production wiring makes at startup.
	require.NoError(t, pgstore.Migrate(ctx, pool))

	scopedDSN = cfg.ConnString()
	if !strings.Contains(scopedDSN, "search_path") {
		sep := "?"
		if strings.Contains(scopedDSN, "?") {
			sep = "&"
		}
		scopedDSN += sep + "search_path=" + schema + ",public"
	}
	return pool, scopedDSN
}

// adminConn returns a process-wide pool on the default search_path, used to
// create and drop per-test schemas.
func adminConn(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(testDBEnv)
	if dsn == "" {
		if v := os.Getenv(requireDBEnv); v != "" && v != "0" && !strings.EqualFold(v, "false") {
			t.Fatalf("%s is set, so the pgpiles suite must run, but %s is empty", requireDBEnv, testDBEnv)
		}
		t.Skipf("%s not set; skipping pgpiles integration tests", testDBEnv)
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
