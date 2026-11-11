//go:build postgres

package jobs_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// neoqTables is the part of neoq's schema the cutover reads and drops, as
// neoq's migrations left it.
const neoqTables = `
CREATE TYPE job_status AS ENUM ('new', 'processed', 'failed');
CREATE TABLE neoq_dead_jobs (id SERIAL NOT NULL, fingerprint text NOT NULL, queue text NOT NULL,
	status job_status NOT NULL default 'failed', payload jsonb, retries integer, max_retries integer,
	created_at timestamptz DEFAULT now(), error text, deadline timestamptz);
CREATE TABLE neoq_jobs (id BIGSERIAL NOT NULL, fingerprint text NOT NULL, queue text NOT NULL,
	status job_status NOT NULL default 'new', payload jsonb, retries integer default 0,
	max_retries integer default 23, run_after timestamptz DEFAULT now(), ran_at timestamptz,
	created_at timestamptz DEFAULT now(), error text, deadline timestamptz);
CREATE TABLE neoq_schema_migrations (version bigint NOT NULL, dirty boolean NOT NULL);
CREATE FUNCTION announce_job() RETURNS trigger AS $$ BEGIN RETURN NEW; END; $$ LANGUAGE plpgsql;
CREATE TRIGGER announce_job AFTER INSERT ON neoq_jobs FOR EACH ROW EXECUTE PROCEDURE announce_job();
`

// keyHash is what the neoq adapter stored as a keyed job's fingerprint: the
// hex SHA-256 of kind, NUL, key.
const keyHash = "7b1a0b0c3c3c7a4d1b0a6e7e1f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e"

// seedNeoq creates neoq's tables in the schema dsn pins and fills them with
// a keyed job, an unkeyed job that failed twice, a processed job, and a
// RetryNever job that failed once and so has no attempt left.
func seedNeoq(t *testing.T, dsn string) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	_, err = pool.Exec(ctx, neoqTables)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO neoq_jobs (fingerprint, queue, status, payload, retries) VALUES
		($1, 'rela', 'new', '{"__rela_kind":"cutover:job","__rela_retry":1,"name":"keyed"}', 0),
		('5f0c6d1e-8a1b-4c2d-9e3f-0a1b2c3d4e5f', 'rela', 'failed',
			'{"__rela_kind":"cutover:job","__rela_retry":1,"name":"retried"}', 2),
		('6a1d7e2f-9b2c-4d3e-8f4a-1b2c3d4e5f60', 'rela', 'processed',
			'{"__rela_kind":"cutover:job","__rela_retry":0,"name":"done"}', 0),
		('7b2e8f30-ac3d-4e4f-9a5b-2c3d4e5f6071', 'rela', 'failed',
			'{"__rela_kind":"cutover:job","__rela_retry":0,"name":"spent"}', 1)`, keyHash)
	require.NoError(t, err)
}

// neoqTablesExist reports whether neoq_jobs is still in schema.
func neoqTablesExist(t *testing.T, admin, schema string) bool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), admin)
	require.NoError(t, err)
	defer pool.Close()
	var exists bool
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT to_regclass($1) IS NOT NULL`,
		pgx.Identifier{schema, "neoq_jobs"}.Sanitize()).Scan(&exists))
	return exists
}

// TestPostgresQueue_MovesNeoqJobs pins the cutover from neoq: pending jobs,
// including one waiting for a retry, run under River with their payload,
// retry intent and attempt count; a processed job and a spent one do not
// run; and neoq's tables are gone.
func TestPostgresQueue_MovesNeoqJobs(t *testing.T) {
	admin := os.Getenv("RELA_TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("RELA_TEST_DATABASE_URL not set; skipping neoq cutover")
	}
	ctx := context.Background()
	const schema = "rela_jobs_cutover_test"
	dsn := schemaPinnedDSN(t, admin, schema)
	seedNeoq(t, dsn)

	q, err := jobs.NewPostgresQueue(ctx, discardLogger(), dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(context.Background()) })
	require.False(t, neoqTablesExist(t, admin, schema), "the cutover must drop neoq's tables")

	var (
		mu  sync.Mutex
		got = map[string]jobs.Job{}
	)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	kind := jobs.Kind("cutover:job")
	require.NoError(t, q.Register(kind, func(ctx context.Context, job jobs.Job) error {
		mu.Lock()
		got[job.Payload["name"].(string)] = job
		mu.Unlock()
		if job.Payload["name"] == "keyed" {
			select { // hold the keyed job so its key stays taken
			case <-release:
			case <-ctx.Done():
			}
		}
		return nil
	}))
	require.NoError(t, q.Start(ctx))

	seen := func(name string) bool {
		mu.Lock()
		defer mu.Unlock()
		_, ok := got[name]
		return ok
	}
	require.Eventually(t, func() bool { return seen("keyed") && seen("retried") }, 10*time.Second, 50*time.Millisecond)

	mu.Lock()
	retried := got["retried"]
	mu.Unlock()
	require.Equal(t, jobs.RetryBounded, retried.Retry)
	require.Equal(t, 3, retried.Attempt, "attempts used under neoq must count")
	require.NotContains(t, retried.Payload, "__rela_kind")
	require.NotContains(t, retried.Payload, "__rela_retry")
	require.False(t, seen("done"), "a processed neoq job must not run again")
	require.False(t, seen("spent"), "a neoq job with no attempt left must not run again")
}

// TestPostgresQueue_NeoqCutoverWaitsForLockedJobs pins that a job a running
// neoq worker holds is not moved, so it cannot run twice: the cutover is
// deferred, the queue still starts, and the next start completes it.
func TestPostgresQueue_NeoqCutoverWaitsForLockedJobs(t *testing.T) {
	admin := os.Getenv("RELA_TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("RELA_TEST_DATABASE_URL not set; skipping neoq cutover")
	}
	ctx := context.Background()
	const schema = "rela_jobs_cutoverlock_test"
	dsn := schemaPinnedDSN(t, admin, schema)
	seedNeoq(t, dsn)

	// A neoq worker holds its job's row lock while the handler runs.
	holder, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer func() { _ = holder.Close(ctx) }()
	tx, err := holder.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT 1 FROM neoq_jobs WHERE fingerprint = $1 FOR UPDATE`, keyHash)
	require.NoError(t, err)

	q, err := jobs.NewPostgresQueue(ctx, discardLogger(), dsn)
	require.NoError(t, err, "a deferred cutover must not stop the queue from starting")
	require.NoError(t, q.Close(ctx))
	require.True(t, neoqTablesExist(t, admin, schema), "locked jobs must not be moved")

	require.NoError(t, tx.Rollback(ctx))
	q, err = jobs.NewPostgresQueue(ctx, discardLogger(), dsn)
	require.NoError(t, err)
	require.NoError(t, q.Close(ctx))
	require.False(t, neoqTablesExist(t, admin, schema), "the next start must finish the cutover")
}
