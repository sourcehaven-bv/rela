package pglease_test

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

	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/lease/leasetest"
	"github.com/Sourcehaven-BV/rela/internal/lease/pglease"
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

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// TestConformance runs the shared backend contract against PostgreSQL, on a
// schema-pinned pool as production uses for a tenant.
func TestConformance(t *testing.T) {
	leasetest.RunAll(t, func(t *testing.T) lease.Store {
		t.Helper()
		s, err := pglease.New(newScopedPool(t))
		require.NoError(t, err)
		return s
	})
}

func TestNewRejectsNilHandle(t *testing.T) {
	_, err := pglease.New(nil)
	require.Error(t, err)
}

// TestSchemaIsolation proves leases are scoped per schema, which is what
// makes schema-per-tenant safe.
func TestSchemaIsolation(t *testing.T) {
	ctx := context.Background()
	a, err := pglease.New(newScopedPool(t))
	require.NoError(t, err)
	b, err := pglease.New(newScopedPool(t))
	require.NoError(t, err)

	h := lease.Holder{Identity: "alice"}
	_, err = a.Claim(ctx, "TKT-1", h, leasetest.Hash("h1"), t0, t0.Add(time.Hour), "")
	require.NoError(t, err)
	_, err = b.Claim(ctx, "TKT-1", h, leasetest.Hash("h2"), t0, t0.Add(time.Hour), "")
	require.NoError(t, err, "a lease in one schema must not block a claim in another")
}

// TestCrossProcess is why this backend exists: two pools, standing in for two
// rela-server processes, grant one lease between them, and the second sees
// the first's version move.
func TestCrossProcess(t *testing.T) {
	ctx := context.Background()
	pool, dsn := newScopedPoolWithDSN(t)
	a, err := pglease.New(pool)
	require.NoError(t, err)
	otherPool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(otherPool.Close)
	b, err := pglease.New(otherPool)
	require.NoError(t, err)

	v, err := b.Version(ctx)
	require.NoError(t, err)
	_, err = a.Claim(ctx, "TKT-1", lease.Holder{Identity: "alice"}, leasetest.Hash("h1"), t0, t0.Add(time.Hour), "")
	require.NoError(t, err)
	v2, err := b.Version(ctx)
	require.NoError(t, err)
	require.NotEqual(t, v, v2, "node B must see node A's write in the version")

	_, err = b.Claim(ctx, "TKT-1", lease.Holder{Identity: "bob"}, leasetest.Hash("h2"), t0, t0.Add(time.Hour), "")
	var held *lease.HeldError
	require.ErrorAs(t, err, &held)
}

// TestReopen_TokenKeepsIncreasing opens a second Store over the same schema
// after a release, at the same instant: only the stored counter can keep the
// token increasing.
func TestReopen_TokenKeepsIncreasing(t *testing.T) {
	ctx := context.Background()
	pool := newScopedPool(t)
	first, err := pglease.New(pool)
	require.NoError(t, err)
	h := lease.Holder{Identity: "alice"}
	l, err := first.Claim(ctx, "TKT-1", h, leasetest.Hash("h1"), t0, t0.Add(time.Hour), "")
	require.NoError(t, err)
	require.NoError(t, first.Release(ctx, leasetest.Hash("h1"), h.Identity, t0))
	require.NoError(t, first.Close())

	second, err := pglease.New(pool)
	require.NoError(t, err)
	l2, err := second.Claim(ctx, "TKT-1", h, leasetest.Hash("h2"), t0, t0.Add(time.Hour), "")
	require.NoError(t, err)
	require.Greater(t, l2.Token, l.Token)
}

func TestStoreCap_CountsLiveLeasesOnly(t *testing.T) {
	ctx := context.Background()
	pool := newScopedPool(t)
	_, err := pool.Exec(ctx, `
		INSERT INTO leases (entity_id, lease_hash, identity, holder_user, holder_tool, label, token, since, expires, status)
		SELECT 'TKT-' || i, 'h-' || i, 'u-' || i, '', '', '', 1, $1,
		       CASE WHEN i = 0 THEN $1::timestamptz + interval '1 minute' ELSE $1::timestamptz + interval '1 hour' END, ''
		FROM generate_series(0, $2 - 1) AS i`, t0, lease.MaxPerStore)
	require.NoError(t, err)
	s, err := pglease.New(pool)
	require.NoError(t, err)

	h := lease.Holder{Identity: "alice"}
	_, err = s.Claim(ctx, "TKT-X", h, leasetest.Hash("hx"), t0, t0.Add(time.Hour), "")
	require.ErrorIs(t, err, lease.ErrFull)

	_, err = s.Claim(ctx, "TKT-X", h, leasetest.Hash("hx"), t0.Add(time.Minute), t0.Add(time.Hour), "")
	require.NoError(t, err, "an expired lease does not count toward the cap")
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

	schema := fmt.Sprintf("relalease_%d_%d", os.Getpid(), schemaCounter.Add(1))
	_, err := admin.Exec(ctx, "CREATE SCHEMA "+quoteIdent(schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoteIdent(schema)+" CASCADE")
	})

	cfg, err := pgxpool.ParseConfig(os.Getenv(testDBEnv))
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	// The concurrency test needs a few connections; the suite builds a pool
	// per subtest, so keep each one small.
	cfg.MaxConns = 4
	cfg.MinConns = 0

	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// The lease tables are part of the pgstore ladder, the same call the
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
			t.Fatalf("%s is set, so the pglease suite must run, but %s is empty", requireDBEnv, testDBEnv)
		}
		t.Skipf("%s not set; skipping pglease integration tests", testDBEnv)
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
