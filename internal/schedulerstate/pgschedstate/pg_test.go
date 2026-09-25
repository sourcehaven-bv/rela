package pgschedstate_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate/pgschedstate"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate/schedulerstatetest"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// testDBEnv and requireDBEnv mirror the pgstore suite's gate: skip by default,
// fail when the environment has promised a database.
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
	schedulerstatetest.RunAll(t, func(t *testing.T) schedulerstate.Store {
		t.Helper()
		store, err := pgschedstate.New(newScopedPool(t))
		require.NoError(t, err)
		return store
	})
}

func TestNewRejectsNilHandle(t *testing.T) {
	_, err := pgschedstate.New(nil)
	require.Error(t, err)
}

// newScopedPool returns a pool pinned to a fresh, migrated schema that is
// dropped when the test ends.
func newScopedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	admin := adminConn(t)
	ctx := context.Background()

	schema := fmt.Sprintf("relasched_%d_%d", os.Getpid(), schemaCounter.Add(1))
	_, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	})

	cfg, err := pgxpool.ParseConfig(os.Getenv(testDBEnv))
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	// Small, because the suite builds a pool per subtest; more than one, so
	// the concurrency cases really race across sessions.
	cfg.MaxConns = 4
	cfg.MinConns = 0

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, pgstore.Migrate(ctx, pool))
	return pool
}

// adminConn returns a process-wide pool used to create and drop schemas.
func adminConn(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(testDBEnv)
	if dsn == "" {
		if v := os.Getenv(requireDBEnv); v != "" && v != "0" && !strings.EqualFold(v, "false") {
			t.Fatalf("%s is set, so the pgschedstate suite must run, but %s is empty", requireDBEnv, testDBEnv)
		}
		t.Skipf("%s not set; skipping pgschedstate suite", testDBEnv)
	}
	adminPoolOnce.Do(func() {
		adminPool, errAdminPool = pgxpool.New(context.Background(), dsn)
	})
	require.NoError(t, errAdminPool)
	return adminPool
}
