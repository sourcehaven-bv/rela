//go:build postgres

package jobs_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"

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
	_, err = pool.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, pgx.Identifier{schema}.Sanitize()))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, pgx.Identifier{schema}.Sanitize()))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, pgx.Identifier{schema}.Sanitize()))
	})

	u, err := url.Parse(admin)
	require.NoError(t, err)
	q := u.Query()
	q.Set("options", "-c search_path="+schema+",public")
	u.RawQuery = q.Encode()

	queue, err := jobs.NewPostgresQueue(ctx, discardLogger(), u.String())
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
