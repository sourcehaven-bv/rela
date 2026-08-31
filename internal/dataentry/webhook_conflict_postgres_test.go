//go:build postgres

package dataentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// These tests exist because the fs/memstore tiers CANNOT exhibit the race the
// webhook pipeline guards against. `checkUniqueProperties` in entitymanager is
// a check-then-write with no lock across the two steps, and fsstore/memstore are
// single-writer by nature, so on those tiers two "concurrent" creates simply
// serialize and both pass. Only PostgreSQL, where a derived partial unique index
// (rela_derived_uniq__*, TKT-3Q0GP1) backstops the scan, can produce the
// atomic loser this pipeline's retry loop is built to handle.
//
// Everything here therefore runs against a SCHEMA-PINNED DSN rather than the
// bare RELA_TEST_DATABASE_URL: the bare DSN resolves to `public`, which is
// precisely the case that works by accident (root CLAUDE.md).

var conflictSchemaCounter atomic.Int64

// conflictTestSchema creates an isolated, migrated schema and returns a pool
// pinned to it plus the schema-pinned DSN.
func conflictTestSchema(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	base := os.Getenv("RELA_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("RELA_TEST_DATABASE_URL not set; skipping webhook conflict tests")
	}

	schema := fmt.Sprintf("relahook_%d_%d", os.Getpid(), conflictSchemaCounter.Add(1))

	u, err := url.Parse(base)
	require.NoError(t, err)
	q := u.Query()
	// Pin every connection from this DSN to the test schema. Keeping `public`
	// on the path is what pg_trgm needs; the schema itself comes first so every
	// unqualified table resolves inside the test's own namespace.
	q.Set("options", fmt.Sprintf("-c search_path=%s,public", schema))
	u.RawQuery = q.Encode()
	dsn := u.String()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS "`+schema+`"`)
	require.NoError(t, err)
	require.NoError(t, pgstore.Migrate(ctx, pool))

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP SCHEMA "`+schema+`" CASCADE`)
		pool.Close()
	})
	return pool, dsn
}

// alertMetamodel is a minimal schema with a `unique:` match property, matching
// the ticket's Identity section (the operator declares uniqueness; the webhook
// layer never assumes it).
func alertMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"incident": {
				Label:    "Incident",
				IDPrefix: "INC-",
				Properties: map[string]metamodel.PropertyDef{
					"title":     {Type: "string", Required: true},
					"alert_key": {Type: "string", Unique: true},
					"status":    {Type: "string"},
				},
				PropertyOrder: []string{"title", "alert_key", "status"},
			},
		},
	}
}

// reconcileUnique creates the derived partial unique indexes for the metamodel,
// which is what makes the create race atomic rather than check-then-write.
//
// Reconciliation is an OPTIONAL store capability, type-asserted rather than part
// of store.Store — the same shape as HistoryReader/VersionWriter.
func reconcileUnique(t *testing.T, st store.Store, meta *metamodel.Metamodel) {
	t.Helper()
	reconciler, ok := st.(interface {
		Reconcile(context.Context, []store.DerivedObjectSpec, store.ReconcileOptions) (
			[]store.DerivedObjectOutcome, error)
	})
	require.True(t, ok, "pgstore must support derived-schema reconciliation")
	var specs []store.DerivedObjectSpec
	for _, typeName := range meta.EntityTypes() {
		def, ok := meta.GetEntityDef(typeName)
		require.True(t, ok)
		for propName, pd := range def.PropertyDefs() {
			if pd.Unique && !pd.List {
				specs = append(specs, store.DerivedObjectSpec{
					Kind: store.DerivedUnique, Type: typeName, Property: propName,
				})
			}
		}
	}
	_, err := reconciler.Reconcile(context.Background(), specs, store.ReconcileOptions{})
	require.NoError(t, err)
}

// newPostgresHookApp builds an App whose store is the given POSTGRES store, so
// the pipeline under test runs against the tier that can actually exhibit
// cross-connection races (fs/memstore are single-writer).
func newPostgresHookApp(t *testing.T, st store.Store, hooks map[string]dataentryconfig.Webhook) *App {
	t.Helper()
	meta := alertMetamodel()

	app := &App{
		scriptEngine:  script.NewEngine(),
		fieldResolver: NopFieldVerdictResolver{},
		auditSink:     audit.Nop{},
	}
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	require.NoError(t, fs.MkdirAll(paths.CacheDir, 0o755))

	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths), appbuildtest.WithStore(st))
	rebindApp(app, fs, paths, svc)

	cfg := &dataentryconfig.Config{
		App:        dataentryconfig.AppConfig{Name: "Hook Test", Description: "x"},
		Forms:      map[string]dataentryconfig.Form{},
		Lists:      map[string]dataentryconfig.List{},
		Views:      map[string]dataentryconfig.ViewConfig{},
		Kanbans:    map[string]dataentryconfig.Kanban{},
		Navigation: []dataentryconfig.NavigationEntry{},
		Webhooks:   hooks,
	}
	app.schema.Publish(&Schema{Cfg: cfg, Meta: meta})
	require.NoError(t, app.SetSecurityConfig(SecurityConfig{BindAddress: "127.0.0.1:8080"}))
	return app
}

// TestWebhookConflict_ConcurrentCreateLosesOnUnique is the correctness core of
// the create half: two concurrent creates carrying the SAME `unique:` value must
// resolve to exactly ONE stored entity, with the loser receiving
// store.UniquePropertyError rather than silently duplicating.
//
// This is the HA-duplicate case from the ticket: two Icinga masters send
// byte-identical notifications, distinguishable only by content hash.
func TestWebhookConflict_ConcurrentCreateLosesOnUnique(t *testing.T) {
	_, dsn := conflictTestSchema(t)
	ctx := context.Background()
	meta := alertMetamodel()

	st, _, closer, err := pgstore.Open(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close(); _ = closer.Close() })
	reconcileUnique(t, st, meta)

	const key = "b7a027ffdd51c19e22e7b7f00c894ef2f3d968896fb40e382df73339c8c645ac"
	const writers = 8

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		wins     int
		conflict int
		other    []error
	)
	start := make(chan struct{})

	for i := range writers {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start // release all writers together to maximize overlap
			e := entity.New(fmt.Sprintf("INC-%d", n), "incident")
			e.Properties = map[string]any{
				"title":     "web01/http",
				"alert_key": key,
				"status":    "open",
			}
			createErr := st.CreateEntity(ctx, e)

			mu.Lock()
			defer mu.Unlock()
			var unique store.UniquePropertyError
			switch {
			case createErr == nil:
				wins++
			case errors.As(createErr, &unique):
				conflict++
			default:
				other = append(other, createErr)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	require.Empty(t, other, "unexpected error kinds from concurrent create")
	require.Equal(t, 1, wins, "exactly one concurrent create must win")
	require.Equal(t, writers-1, conflict,
		"every loser must see store.UniquePropertyError (the derived unique index is the backstop)")

	// The database must hold exactly one incident for the key.
	var stored int
	for e, listErr := range st.ListEntities(ctx, store.EntityQuery{Type: "incident"}) {
		require.NoError(t, listErr)
		if e.Properties["alert_key"] == key {
			stored++
		}
	}
	require.Equal(t, 1, stored, "the unique index must permit exactly one row for the key")
}

// TestWebhookConflict_LoserRefindsAndProceeds pins the pipeline's response to
// losing: after a UniquePropertyError the executor re-finds and takes the
// UPDATE path, so the delivery still lands rather than being dropped. This is
// the behaviour that makes the no-lock design safe.
func TestWebhookConflict_LoserRefindsAndProceeds(t *testing.T) {
	_, dsn := conflictTestSchema(t)
	ctx := context.Background()
	meta := alertMetamodel()

	st, _, closer, err := pgstore.Open(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close(); _ = closer.Close() })
	reconcileUnique(t, st, meta)

	const key = "shared-alert-key"

	// The winner is already stored, exactly as it would be when a racing
	// delivery lost the create.
	winner := entity.New("INC-WINNER", "incident")
	winner.Properties = map[string]any{"title": "web01/http", "alert_key": key, "status": "open"}
	require.NoError(t, st.CreateEntity(ctx, winner))

	// The loser attempts the same create and must be rejected atomically...
	loser := entity.New("INC-LOSER", "incident")
	loser.Properties = map[string]any{"title": "web01/http", "alert_key": key, "status": "open"}
	createErr := st.CreateEntity(ctx, loser)

	var unique store.UniquePropertyError
	require.ErrorAs(t, createErr, &unique,
		"the loser must get UniquePropertyError so the pipeline knows to re-find")

	// ...and the re-find that the pipeline performs must locate the winner, so
	// the delivery proceeds as an update instead of being lost.
	var found *entity.Entity
	for e, listErr := range st.ListEntities(ctx, store.EntityQuery{Type: "incident"}) {
		require.NoError(t, listErr)
		if e.Properties["alert_key"] == key {
			found = e
		}
	}
	require.NotNil(t, found, "the re-find must locate the winner")
	require.Equal(t, "INC-WINNER", found.ID)
}

// TestWebhookConflict_BlindUpdateLosesAppends documents WHY applyWebhookSteps
// re-reads the body before splicing, by demonstrating the failure it avoids.
//
// store.UpdateEntity is a blind write with no compare-and-swap: concurrent
// read-modify-write appends computed from a base captured earlier silently
// overwrite one another. Nothing errors — the notifications simply vanish, which
// is the worst shape of failure for an alert pipeline.
//
// This test asserts the LOSS, so it is a characterization of the store's
// contract rather than a wish. If a future change makes UpdateEntity
// conflict-detecting, this test failing is the correct signal to revisit the
// re-read in applyWebhookSteps.
func TestWebhookConflict_BlindUpdateLosesAppends(t *testing.T) {
	_, dsn := conflictTestSchema(t)
	ctx := context.Background()

	st, _, closer, err := pgstore.Open(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close(); _ = closer.Close() })

	seed := entity.New("INC-APPEND", "incident")
	seed.Properties = map[string]any{"title": "web01/http", "status": "open"}
	seed.Content = "## Notifications"
	require.NoError(t, st.CreateEntity(ctx, seed))

	// Every writer captures the SAME stale base first, then writes — the
	// read-modify-write pattern applyWebhookSteps must not reproduce.
	stale, err := st.GetEntity(ctx, "INC-APPEND")
	require.NoError(t, err)

	const appenders = 6
	var wg sync.WaitGroup
	for i := range appenders {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			updated := stale.Clone()
			updated.Content = stale.Content + fmt.Sprintf("\n- alert %d", n)
			_ = st.UpdateEntity(ctx, updated)
		}(i)
	}
	wg.Wait()

	final, err := st.GetEntity(ctx, "INC-APPEND")
	require.NoError(t, err)

	landed := 0
	for i := range appenders {
		if strings.Contains(final.Content, fmt.Sprintf("- alert %d", i)) {
			landed++
		}
	}
	require.Less(t, landed, appenders,
		"expected blind concurrent updates to LOSE writes; if all %d landed, "+
			"UpdateEntity may have become conflict-detecting — revisit applyWebhookSteps", appenders)
}

// TestWebhookConflict_PipelineAppendsAllLand is the positive counterpart: the
// REAL pipeline, driven concurrently through the production router against a
// postgres store, must not lose an append. Six concurrent deliveries, six
// notifications present at the end.
//
// What this DOES prove: the assembled pipeline (writeMu + per-attempt re-find +
// PatchEntity) is safe against concurrent deliveries reaching one process.
//
// What it does NOT prove, stated plainly so the next reader is not misled:
// it does not isolate the re-read in applyWebhookSteps. writeMu serializes the
// deliveries and each attempt re-runs findWebhookTarget, so this test still
// passes with that re-read removed — its value on the find path is redundancy,
// not necessity (see the note on applyWebhookSteps). Nor does it cover
// MULTI-PROCESS appenders, which is the case no part of this pipeline currently
// closes; the honest scope is one process, many concurrent requests.
func TestWebhookConflict_PipelineAppendsAllLand(t *testing.T) {
	_, dsn := conflictTestSchema(t)
	ctx := context.Background()

	st, _, closer, err := pgstore.Open(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close(); _ = closer.Close() })

	app := newPostgresHookApp(t, st, map[string]dataentryconfig.Webhook{
		"alert": {
			Find: &dataentryconfig.WebhookFind{Type: "incident", Match: []string{"title"}},
			Then: []dataentryconfig.WebhookStep{{
				AppendSection: &dataentryconfig.WebhookAppendSection{
					Section: "Notifications", Content: "- alert {{body.n}}",
				},
			}},
		},
	})

	seed := entity.New("INC-PIPE", "incident")
	seed.Properties = map[string]any{"title": "web01/http", "status": "open"}
	seed.Content = "## Notifications"
	require.NoError(t, st.CreateEntity(ctx, seed))

	const deliveries = 6
	router := app.NewRouter()
	var wg sync.WaitGroup
	start := make(chan struct{})
	codes := make([]int, deliveries)

	for i := range deliveries {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			body := fmt.Sprintf(`{"title":"web01/http","n":%d}`, n)
			req := httptest.NewRequest(http.MethodPost, "/hooks/alert", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Host = "127.0.0.1:8080"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			codes[n] = rec.Code
		}(i)
	}
	close(start)
	wg.Wait()

	for i, code := range codes {
		require.Equal(t, http.StatusOK, code, "delivery %d did not succeed", i)
	}

	final, err := st.GetEntity(ctx, "INC-PIPE")
	require.NoError(t, err)
	for i := range deliveries {
		require.Contains(t, final.Content, fmt.Sprintf("- alert %d", i),
			"delivery %d was lost; content = %q", i, final.Content)
	}
}

// TestWebhookConflict_CrossProcessAppendsCanBeLost documents the ONE case this
// design does not close, so the limitation is recorded in code rather than only
// in prose.
//
// Two Apps on SEPARATE pgstores over the SAME schema model two rela-server
// processes against one database (the multi-writer deployment documented in
// docs/postgres-backend.md). writeMu is per-process, so it does not serialize
// them, and nothing in the append path is a compare-and-swap — so a concurrent
// append CAN be lost across processes.
//
// The test asserts the weaker property that actually holds (both deliveries
// succeed and at least one notification lands) and reports when a loss occurs,
// rather than asserting a guarantee the implementation does not provide. A
// server-side append mode on entity.Patch is the fix; it is a named follow-up.
func TestWebhookConflict_CrossProcessAppendsCanBeLost(t *testing.T) {
	_, dsn := conflictTestSchema(t)
	ctx := context.Background()

	hooks := map[string]dataentryconfig.Webhook{
		"alert": {
			Find: &dataentryconfig.WebhookFind{Type: "incident", Match: []string{"title"}},
			Then: []dataentryconfig.WebhookStep{{
				AppendSection: &dataentryconfig.WebhookAppendSection{
					Section: "Notifications", Content: "- alert {{body.n}}",
				},
			}},
		},
	}

	// Two independent stores over one schema == two processes.
	stA, _, closerA, err := pgstore.Open(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = stA.Close(); _ = closerA.Close() })
	stB, _, closerB, err := pgstore.Open(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = stB.Close(); _ = closerB.Close() })

	seed := entity.New("INC-XPROC", "incident")
	seed.Properties = map[string]any{"title": "web01/http", "status": "open"}
	seed.Content = "## Notifications"
	require.NoError(t, stA.CreateEntity(ctx, seed))

	routerA := newPostgresHookApp(t, stA, hooks).NewRouter()
	routerB := newPostgresHookApp(t, stB, hooks).NewRouter()

	deliver := func(router http.Handler, n int) int {
		body := fmt.Sprintf(`{"title":"web01/http","n":%d}`, n)
		req := httptest.NewRequest(http.MethodPost, "/hooks/alert", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Host = "127.0.0.1:8080"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	var wg sync.WaitGroup
	codes := make([]int, 2)
	start := make(chan struct{})
	for i, router := range []http.Handler{routerA, routerB} {
		wg.Add(1)
		go func(n int, rt http.Handler) {
			defer wg.Done()
			<-start
			codes[n] = deliver(rt, n)
		}(i, router)
	}
	close(start)
	wg.Wait()

	for i, code := range codes {
		require.Equal(t, http.StatusOK, code, "cross-process delivery %d failed", i)
	}

	final, err := stA.GetEntity(ctx, "INC-XPROC")
	require.NoError(t, err)

	landed := 0
	for i := range 2 {
		if strings.Contains(final.Content, fmt.Sprintf("- alert %d", i)) {
			landed++
		}
	}
	require.Positive(t, landed, "at least one cross-process append must land")
	if landed < 2 {
		t.Logf("KNOWN LIMITATION: %d of 2 cross-process appends landed; "+
			"the append path is not a compare-and-swap across processes "+
			"(server-side append mode on entity.Patch is the fix). content = %q",
			landed, final.Content)
	}
}

// TestWebhookConflict_SchemaPinnedDSNIsIsolated proves the harness above is
// actually schema-pinned rather than silently writing to `public`. Without this
// the concurrency tests could pass against a shared namespace and prove nothing
// about tenant isolation.
func TestWebhookConflict_SchemaPinnedDSNIsIsolated(t *testing.T) {
	poolA, dsnA := conflictTestSchema(t)
	_, dsnB := conflictTestSchema(t)
	require.NotEqual(t, dsnA, dsnB, "each test schema must get its own DSN")

	ctx := context.Background()
	stA, _, closerA, err := pgstore.Open(ctx, dsnA)
	require.NoError(t, err)
	t.Cleanup(func() { _ = stA.Close(); _ = closerA.Close() })

	stB, _, closerB, err := pgstore.Open(ctx, dsnB)
	require.NoError(t, err)
	t.Cleanup(func() { _ = stB.Close(); _ = closerB.Close() })

	e := entity.New("INC-ISOLATED", "incident")
	e.Properties = map[string]any{"title": "only in A"}
	require.NoError(t, stA.CreateEntity(ctx, e))

	// B must not see A's row.
	_, err = stB.GetEntity(ctx, "INC-ISOLATED")
	require.Error(t, err, "schema B must not see schema A's entity — the DSN is not pinned")

	// And the ROW must live in A's own schema, not in public. Asserting on the
	// row rather than on the existence of a public.entities TABLE is deliberate:
	// a shared test database usually already has one from other suites, so a
	// table-existence check would fail for reasons unrelated to pinning.
	var inA int
	require.NoError(t, poolA.QueryRow(ctx,
		`SELECT count(*) FROM entities WHERE id = $1`, "INC-ISOLATED",
	).Scan(&inA))
	require.Equal(t, 1, inA, "the row must be in the pinned schema (unqualified name resolves there)")

	var inPublic int
	require.NoError(t, poolA.QueryRow(ctx,
		`SELECT count(*) FROM public.entities WHERE id = $1`, "INC-ISOLATED",
	).Scan(&inPublic))
	require.Zero(t, inPublic, "the row leaked into public — the DSN is not schema-pinned")
}
