package pgcomments_test

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

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/commentstest"
	"github.com/Sourcehaven-BV/rela/internal/comments/pgcomments"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// testDBEnv and requireDBEnv mirror the pgstore suite's gate, deliberately.
//
// A skip and a pass are indistinguishable in `go test` exit codes, so a suite
// that silently skips when the DSN is missing stops being a gate at the moment
// it is needed most (RR-0EWZQW records two real defects that shipped past every
// non-DB check for exactly this reason). The default is still a clean skip, so
// a plain `go test ./...` needs no database; CI's Postgres job sets both and
// therefore MUST run this.
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

// TestConformance runs the shared backend contract against PostgreSQL.
//
// This is the whole point of commentstest existing: the ordering, aliasing and
// concurrency rules that differ subtly between backends are asserted once, and
// a new backend either satisfies them or does not.
func TestConformance(t *testing.T) {
	commentstest.RunAll(t, func(t *testing.T) comments.Store {
		t.Helper()
		store, err := pgcomments.New(newScopedPool(t))
		require.NoError(t, err)
		return store
	})
}

// TestKeyFidelity holds this backend to byte-exact target keys, which RunAll
// deliberately leaves out: filecomments cannot promise case-sensitivity on a
// case-insensitive filesystem and refuses non-ASCII ids outright, both
// reasonably. A database backend has neither excuse.
func TestKeyFidelity(t *testing.T) {
	commentstest.RunKeyFidelityTests(t, func(t *testing.T) comments.Store {
		t.Helper()
		store, err := pgcomments.New(newScopedPool(t))
		require.NoError(t, err)
		return store
	})
}

// TestNewRejectsNilHandle pins the constructor's nil contract: a wiring mistake
// must fail at construction, not at the first comment someone posts.
func TestNewRejectsNilHandle(t *testing.T) {
	_, err := pgcomments.New(nil)
	require.Error(t, err)
}

// TestSchemaIsolation proves comments are scoped per schema, which is what
// makes schema-per-tenant safe: two tenants on one database must not see each
// other's commentary.
func TestSchemaIsolation(t *testing.T) {
	ctx := context.Background()
	a, err := pgcomments.New(newScopedPool(t))
	require.NoError(t, err)
	b, err := pgcomments.New(newScopedPool(t))
	require.NoError(t, err)

	target := comments.Target{Type: "ticket", ID: "TKT-1"}
	require.NoError(t, a.Add(ctx, target, comments.Comment{
		ID:     "c1",
		Author: "alice",
		Anchor: comments.Anchor{Kind: comments.AnchorProperty, Ref: "status"},
		Body:   "only in schema A",
	}))

	got, err := b.List(ctx, target)
	require.NoError(t, err)
	require.Empty(t, got, "a comment in one schema must be invisible in another")
}

// TestCrossProcessVisibility is the defect this backend exists to fix: two
// independent pools (standing in for two rela-server processes) against ONE
// schema must see each other's comments. With filecomments they would not,
// because each node writes its own .rela/comments/ directory.
func TestCrossProcessVisibility(t *testing.T) {
	ctx := context.Background()
	pool, dsn := newScopedPoolWithDSN(t)

	writer, err := pgcomments.New(pool)
	require.NoError(t, err)

	// A genuinely separate pool, as a second process would have.
	readerPool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(readerPool.Close)
	reader, err := pgcomments.New(readerPool)
	require.NoError(t, err)

	target := comments.Target{Type: "ticket", ID: "TKT-1"}
	require.NoError(t, writer.Add(ctx, target, comments.Comment{
		ID:     "c1",
		Author: "alice",
		Anchor: comments.Anchor{Kind: comments.AnchorProperty, Ref: "status"},
		Body:   "posted through node A",
	}))

	got, err := reader.List(ctx, target)
	require.NoError(t, err)
	require.Len(t, got, 1, "node B must see a comment posted through node A")
	require.Equal(t, "posted through node A", got[0].Body)
}

// TestRenameDoesNotMatchLikeWildcards pins the escaping in facePrefixPattern.
//
// An entity id may contain "_" (entity.ValidateID admits [A-Za-z0-9_-]), which
// is LIKE's single-character wildcard. Unescaped, renaming "TKT_1" would also
// re-key "TKT-1" and silently merge two unrelated threads.
func TestRenameDoesNotMatchLikeWildcards(t *testing.T) {
	ctx := context.Background()
	store, err := pgcomments.New(newScopedPool(t))
	require.NoError(t, err)

	add := func(target comments.Target, id string) {
		require.NoError(t, store.Add(ctx, target, comments.Comment{
			ID:     id,
			Author: "alice",
			Anchor: comments.Anchor{Kind: comments.AnchorProperty, Ref: "status"},
			Body:   "body",
		}))
	}
	// Faced, so the threads are reachable only through the prefix pattern.
	add(comments.Target{Type: "ticket", ID: "TKT_1", Face: "draft"}, "under")
	add(comments.Target{Type: "ticket", ID: "TKT-1", Face: "draft"}, "hyphen")

	require.NoError(t, store.Rename(ctx, "TKT_1", "TKT-9"))

	// The hyphen thread must be untouched by the underscore rename.
	got, err := store.List(ctx, comments.Target{Type: "ticket", ID: "TKT-1", Face: "draft"})
	require.NoError(t, err)
	require.Equal(t, []string{"hyphen"}, idsOf(got),
		"renaming TKT_1 must not re-key TKT-1: _ is a LIKE wildcard")

	moved, err := store.List(ctx, comments.Target{Type: "ticket", ID: "TKT-9", Face: "draft"})
	require.NoError(t, err)
	require.Equal(t, []string{"under"}, idsOf(moved))
}

func idsOf(list []comments.Comment) []string {
	out := make([]string, len(list))
	for i, c := range list {
		out[i] = c.ID
	}
	return out
}

// newScopedPool returns a pool pinned to a fresh, migrated schema.
func newScopedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, _ := newScopedPoolWithDSN(t)
	return pool
}

// newScopedPoolWithDSN also returns a DSN pinned to the same schema, for tests
// that need a second, independent pool against it.
func newScopedPoolWithDSN(t *testing.T) (pool *pgxpool.Pool, scopedDSN string) {
	t.Helper()
	admin := adminConn(t)
	ctx := context.Background()

	schema := fmt.Sprintf("relacmt_%d_%d", os.Getpid(), schemaCounter.Add(1))
	_, err := admin.Exec(ctx, "CREATE SCHEMA "+quoteIdent(schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoteIdent(schema)+" CASCADE")
	})

	cfg, err := pgxpool.ParseConfig(os.Getenv(testDBEnv))
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	// The conformance suite builds a pool per subtest; keep each small so the
	// suite stays well under the server's max_connections.
	cfg.MaxConns = 2
	cfg.MinConns = 0

	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// The comments table is part of the pgstore ladder, so migrating the schema
	// creates it — the same call the production wiring makes at startup.
	require.NoError(t, pgstore.Migrate(ctx, pool))

	scopedDSN = cfg.ConnString()
	if !strings.Contains(scopedDSN, "search_path") {
		scopedDSN = appendSearchPath(scopedDSN, schema)
	}
	return pool, scopedDSN
}

// appendSearchPath pins a DSN to one schema, so a second pool built from it
// lands in the same place as the first.
func appendSearchPath(dsn, schema string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "search_path=" + schema + ",public"
}

// adminConn returns a process-wide pool on the default search_path, used to
// create and drop per-test schemas.
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

// skipOrFailWithoutDSN skips by default, but FAILS when the environment has
// promised a database. See the const block above for why the distinction
// matters.
func skipOrFailWithoutDSN(t *testing.T) {
	t.Helper()
	if v := os.Getenv(requireDBEnv); v != "" && v != "0" && !strings.EqualFold(v, "false") {
		t.Fatalf("%s is set, so the pgcomments suite must run, but %s is empty.\n"+
			"This suite is the comment-backend parity gate: skipping it silently would let "+
			"file-vs-database divergence merge unnoticed.\n"+
			"Set %s to a reachable PostgreSQL DSN, or unset %s to allow skipping.",
			requireDBEnv, testDBEnv, testDBEnv, requireDBEnv)
	}
	t.Skipf("%s not set; skipping pgcomments integration tests", testDBEnv)
}

// quoteIdent quotes a SQL identifier. Schema names here are generated, but
// quoting is not optional hygiene — an unquoted identifier is a different bug
// class the moment the generator changes.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
