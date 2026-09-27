//go:build postgres

package jobs_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/jobs/jobstest"
)

// TestPostgresQueue_Conformance runs the shared suite against the durable
// backend.
//
// This exists because a memory-only suite cannot see backend-specific failures,
// and one shipped: the idempotency fingerprint used a NUL byte as a separator,
// which Go and the memory backend accept happily and PostgreSQL rejects
// outright ("invalid byte sequence for encoding UTF8: 0x00"). Every scheduled
// job on the durable tier failed to enqueue, with the whole suite green. Only
// an end-to-end run against a real database found it.
//
// Gated on RELA_TEST_DATABASE_URL and skipped when unset, matching how
// pgstore's suite is gated — run it with `just test-postgres`.
func TestPostgresQueue_Conformance(t *testing.T) {
	dsn := os.Getenv("RELA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("RELA_TEST_DATABASE_URL not set; skipping postgres job queue conformance")
	}

	jobstest.RunAll(t, func(t *testing.T) jobs.Queue {
		t.Helper()
		q, err := jobs.NewPostgresQueue(context.Background(), discardLogger(), dsn)
		require.NoError(t, err)
		t.Cleanup(func() { _ = q.Close(context.Background()) })
		return q
	})
}

func TestNewPostgresQueue_RejectsEmptyDSN(t *testing.T) {
	_, err := jobs.NewPostgresQueue(context.Background(), discardLogger(), "")
	require.Error(t, err)
}

// TestPostgresQueue_SchemaPinnedDSN pins that the durable queue initializes
// against a DSN whose search_path points somewhere other than `public`.
//
// This is how rela scopes a tenant — the same search_path that scopes
// entities, relations and state_kv is expected to scope the queue's tables —
// and it is how the postgres e2e specs connect. It did NOT work: neoq's
// make-job-id-bigint migration named `public.neoq_jobs_id_seq` while its
// tables are created through search_path, so every start against a pinned
// schema failed with `relation "public.neoq_jobs_id_seq" does not exist` and
// the server never came up. Fixed in the neoq fork pinned by go.mod's replace
// directive; this test is what notices if that replace is ever dropped.
//
// The conformance suite above missed it because it passes RELA_TEST_DATABASE_URL
// through unchanged, which resolves to `public`.
func TestPostgresQueue_SchemaPinnedDSN(t *testing.T) {
	admin := os.Getenv("RELA_TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("RELA_TEST_DATABASE_URL not set; skipping schema-pinned queue init")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, admin)
	require.NoError(t, err)
	defer pool.Close()

	// A schema name unique to this test, created empty so the queue has to run
	// neoq's migrations from scratch — the state the bug fired in.
	const schema = "rela_jobs_schemapin_test"
	dsn := schemaPinnedDSN(t, admin, schema)

	queue, err := jobs.NewPostgresQueue(ctx, discardLogger(), dsn)
	require.NoError(t, err, "queue must initialize against a schema-pinned DSN")
	t.Cleanup(func() { _ = queue.Close(context.Background()) })

	// The tables must land in the pinned schema, not in public: rela submits
	// every kind to one queue name and neoq's insert trigger does
	// pg_notify(NEW.queue), so tables shared across tenants would mean tenants
	// consuming each other's jobs.
	var inSchema bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables
		   WHERE table_schema = $1 AND table_name = 'neoq_jobs')`, schema,
	).Scan(&inSchema))
	require.True(t, inSchema, "neoq_jobs must be created in the pinned schema")
}

// TestPostgresQueue_HandlerOutlivesDefaultIdleTxTimeout pins BUG-TKL08E.
//
// neoq runs a handler inside the transaction holding the job's row lock, and
// its default idle_in_transaction_session_timeout is 30 s. A handler longer
// than that had its session killed underneath it: the outcome update failed,
// the row stayed pending, neoq redelivered it every minute forever, and its
// idempotency key rejected every later enqueue. On Atlas that stalled the
// scheduler until a restart.
//
// Uses the production constructor and a handler just over the old 30 s limit,
// because the defect was the absence of an option, and only the real value
// shows it is present. Slow by nature, so skipped under -short.
func TestPostgresQueue_HandlerOutlivesDefaultIdleTxTimeout(t *testing.T) {
	dsn := os.Getenv("RELA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("RELA_TEST_DATABASE_URL not set; skipping postgres long-handler case")
	}
	if testing.Short() {
		t.Skip("needs a handler longer than neoq's 30 s default; run without -short")
	}
	ctx := context.Background()

	q, err := jobs.NewPostgresQueue(ctx, discardLogger(), schemaPinnedDSN(t, dsn, "rela_jobs_longhandler_test"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(ctx) })

	const longRun = 35 * time.Second
	kind := jobs.NewKind("jobstest", "long")
	var longDeliveries, shortRuns atomic.Int32
	require.NoError(t, q.Register(kind, func(_ context.Context, job jobs.Job) error {
		if job.Payload["long"] == true {
			longDeliveries.Add(1)
			time.Sleep(longRun)
			return nil
		}
		shortRuns.Add(1)
		return nil
	}))
	require.NoError(t, q.Start(ctx))

	started := time.Now()
	require.NoError(t, q.Enqueue(ctx, jobs.Job{
		Kind: kind, Retry: jobs.RetryNever, IdempotencyKey: "long",
		Payload: map[string]any{"long": true},
	}))

	// The key frees only once neoq has COMMITTED the outcome, which is the step
	// the killed session could never complete.
	//
	// Bounded well below neoq's 60 s pending-job poll. Without the fix the key
	// still frees eventually: the poll redelivers the stranded row, and that
	// rerun commits. So a generous bound would pass on the broken code, and
	// the first delivery committing is only visible as the key freeing BEFORE
	// any redelivery was possible.
	short := jobs.Job{Kind: kind, Retry: jobs.RetryNever, IdempotencyKey: "long"}
	require.Eventually(t, func() bool { return q.Enqueue(ctx, short) == nil },
		longRun+15*time.Second-time.Since(started), 250*time.Millisecond,
		"a handler longer than 30 s must commit its outcome on the first delivery")
	require.Eventually(t, func() bool { return shortRuns.Load() == 1 }, 10*time.Second, 50*time.Millisecond)
	require.Equal(t, int32(1), longDeliveries.Load(), "the long job must not be redelivered")
}

// schemaPinnedDSN creates an empty schema, drops it when the test ends, and
// returns admin's DSN with its search_path pinned there.
func schemaPinnedDSN(t *testing.T, admin, schema string) string {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, admin)
	require.NoError(t, err)
	defer pool.Close()

	_, err = pool.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, pgx.Identifier{schema}.Sanitize()))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, pgx.Identifier{schema}.Sanitize()))
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, err := pgxpool.New(context.Background(), admin)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.Exec(context.Background(),
			fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, pgx.Identifier{schema}.Sanitize()))
	})

	// Pin the schema with a plain `search_path` runtime parameter rather than
	// libpq's `options=-c search_path=...`. The latter has to survive
	// url.Values.Encode, which writes its space as "+" — and a URI query means
	// a literal plus there, so libpq (and pgx from v5.11.0 on) reads the
	// parameter name as "+search_path" and the server refuses the connection.
	// `search_path` as its own parameter has no space to get wrong, and keeps
	// the DSN a URL so ensurePoolFloor still sizes the pool.
	u, err := url.Parse(admin)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	return u.String()
}
