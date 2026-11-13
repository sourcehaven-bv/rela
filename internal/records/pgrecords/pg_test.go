package pgrecords_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/pgrecords"
	"github.com/Sourcehaven-BV/rela/internal/records/recordstest"
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

func TestConformance(t *testing.T) {
	recordstest.RunAll(t, func(t *testing.T) records.Store {
		t.Helper()
		s, err := pgrecords.New(newScopedPool(t))
		require.NoError(t, err)
		return s
	})
}

func TestNewRejectsNil(t *testing.T) {
	_, err := pgrecords.New(nil)
	require.Error(t, err)
}

// TestSchemaIsolation: rows and group claims of one tenant schema are
// invisible in another.
func TestSchemaIsolation(t *testing.T) {
	ctx := context.Background()
	a, err := pgrecords.New(newScopedPool(t))
	require.NoError(t, err)
	b, err := pgrecords.New(newScopedPool(t))
	require.NoError(t, err)
	r := recordstest.Row("a", "l", "O", 0, nil)
	r.Group = "tx"
	req := records.AppendRequest{Records: []records.Record{r}, Groups: []records.GroupKey{{Type: "l", Group: "tx"}}}
	require.NoError(t, a.Append(ctx, req))
	require.NoError(t, b.Append(ctx, req), "the claim in schema a does not hold in schema b")
	owners, err := b.Owners(ctx, "l")
	require.NoError(t, err)
	require.Equal(t, []string{"O"}, owners)
}

// newScopedPool returns a pool pinned to a fresh, migrated schema.
func newScopedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	admin := adminConn(t)
	ctx := context.Background()
	schema := fmt.Sprintf("relarec_%d_%d", os.Getpid(), schemaCounter.Add(1))
	_, err := admin.Exec(ctx, "CREATE SCHEMA "+quoteIdent(schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoteIdent(schema)+" CASCADE")
	})
	cfg, err := pgxpool.ParseConfig(os.Getenv(testDBEnv))
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	cfg.MaxConns = 8
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pgstore.Migrate(ctx, pool))
	return pool
}

func adminConn(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(testDBEnv)
	if dsn == "" {
		if v := os.Getenv(requireDBEnv); v != "" && v != "0" && !strings.EqualFold(v, "false") {
			t.Fatalf("%s is set, so the pgrecords suite must run, but %s is empty", requireDBEnv, testDBEnv)
		}
		t.Skipf("%s not set; skipping pgrecords integration tests", testDBEnv)
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
