package pgmigstate_test

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

	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/migstatetest"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/pgmigstate"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// testDBEnv and requireDBEnv mirror the pgstore and pgcomments gates.
//
// A skip and a pass are indistinguishable in `go test` exit codes, so a suite
// that silently skips when the DSN is missing stops being a gate at the moment
// it matters. The default is a clean skip so `go test ./...` needs no
// database; CI's Postgres job sets both and therefore must run this.
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

func TestConformance(t *testing.T) {
	migstatetest.RunAll(t, func(t *testing.T) datamigration.StateStore {
		t.Helper()
		st, err := pgmigstate.New(newScopedPool(t))
		require.NoError(t, err)
		return st
	})
}

func TestNewRejectsNilHandle(t *testing.T) {
	_, err := pgmigstate.New(nil)
	require.Error(t, err, "a no-op store would report every migration un-run and replay the chain")
}

// TestTenantIsolation is the guarantee this backend exists to keep.
//
// docs/postgres-backend.md documents that each tenant tracks the shape its own
// data conforms to, "so tenants at different points migrate independently".
// That is precisely what one shared committed file could not express, and it
// is why the migration record is backend-selected rather than always a file.
func TestTenantIsolation(t *testing.T) {
	ctx := context.Background()
	a, err := pgmigstate.New(newScopedPool(t))
	require.NoError(t, err)
	b, err := pgmigstate.New(newScopedPool(t))
	require.NoError(t, err)

	now := time.Date(2026, 9, 19, 14, 30, 22, 0, time.UTC)
	st, err := datamigration.NewState(fixtureProjection(), []datamigration.AppliedEntry{
		{Name: "20260919143022-only-in-tenant-a.yaml", AppliedAt: now},
	}, now)
	require.NoError(t, err)
	require.NoError(t, a.Save(ctx, st))

	// Tenant B is untouched: it has run nothing and must still read as
	// un-bootstrapped, not inherit A's position.
	got, err := b.Load(ctx)
	require.NoError(t, err)
	require.Nil(t, got, "one tenant's migration record must be invisible to another")

	// And A still reads its own.
	gotA, err := a.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"20260919143022-only-in-tenant-a.yaml"}, gotA.AppliedNames())
}

// TestCrossProcessVisibility: two independent pools standing in for two
// rela-server processes against ONE schema must agree on what has run. This is
// the defect the file backend would have on this tier — each node would keep
// its own answer and replay migrations the others had already applied.
func TestCrossProcessVisibility(t *testing.T) {
	ctx := context.Background()
	pool, dsn := newScopedPoolWithDSN(t)

	writer, err := pgmigstate.New(pool)
	require.NoError(t, err)

	readerPool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(readerPool.Close)
	reader, err := pgmigstate.New(readerPool)
	require.NoError(t, err)

	now := time.Date(2026, 9, 19, 14, 30, 22, 0, time.UTC)
	st, err := datamigration.NewState(fixtureProjection(), []datamigration.AppliedEntry{
		{Name: "20260919143022-shared.yaml", AppliedAt: now},
	}, now)
	require.NoError(t, err)
	require.NoError(t, writer.Save(ctx, st))

	got, err := reader.Load(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, []string{"20260919143022-shared.yaml"}, got.AppliedNames())
}

// A row that exists but cannot be parsed must ERROR, never read as absent:
// absent means un-bootstrapped, which baselines the store and marks every
// pending migration applied.
func TestLoadRejectsMalformedRow(t *testing.T) {
	ctx := context.Background()
	pool := newScopedPool(t)
	st, err := pgmigstate.New(pool)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO migration_state (id, state) VALUES (1, '{"format_version": 9999}'::jsonb)`)
	require.NoError(t, err)

	got, err := st.Load(ctx)
	require.Error(t, err, "state from a newer format must be refused, not treated as absent")
	require.Nil(t, got)
	require.Contains(t, err.Error(), "upgrade rela")
}

func fixtureProjection() metamodel.ShapeProjection {
	m := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"task": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true},
			}},
		},
	}
	return m.ShapeProjection()
}

func newScopedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, _ := newScopedPoolWithDSN(t)
	return pool
}

func newScopedPoolWithDSN(t *testing.T) (pool *pgxpool.Pool, scopedDSN string) {
	t.Helper()
	admin := adminConn(t)
	ctx := context.Background()

	schema := fmt.Sprintf("relamig_%d_%d", os.Getpid(), schemaCounter.Add(1))
	_, err := admin.Exec(ctx, "CREATE SCHEMA "+quoteIdent(schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoteIdent(schema)+" CASCADE")
	})

	cfg, err := pgxpool.ParseConfig(os.Getenv(testDBEnv))
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	cfg.MaxConns = 2
	cfg.MinConns = 0

	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// migration_state is part of the pgstore ladder, so migrating the schema
	// creates it — the same call the production wiring makes at startup.
	require.NoError(t, pgstore.Migrate(ctx, pool))

	scopedDSN = cfg.ConnString()
	if !strings.Contains(scopedDSN, "search_path") {
		scopedDSN = appendSearchPath(scopedDSN, schema)
	}
	return pool, scopedDSN
}

func appendSearchPath(dsn, schema string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "search_path=" + schema + ",public"
}

func adminConn(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(testDBEnv)
	if dsn == "" {
		skipOrFailWithoutDSN(t)
		return nil
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

func skipOrFailWithoutDSN(t *testing.T) {
	t.Helper()
	if v := os.Getenv(requireDBEnv); v != "" && v != "0" && !strings.EqualFold(v, "false") {
		t.Fatalf("%s is set, so the pgmigstate suite must run, but %s is empty.\n"+
			"This suite is the migration-record parity gate: skipping it silently would let "+
			"file-vs-database divergence merge unnoticed.\n"+
			"Set %s to a reachable PostgreSQL DSN, or unset %s to allow skipping.",
			requireDBEnv, testDBEnv, testDBEnv, requireDBEnv)
	}
	t.Skipf("%s not set; skipping pgmigstate integration tests", testDBEnv)
}
