//go:build postgres

package jobs_test

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// TestPostgresQueue_SurvivesAnotherProcessClosing pins that a queue keeps
// processing when another process sharing the database closes its own queue.
//
// neoq's Shutdown used to pg_notify a sentinel job ID on the queue channel to
// stop its listener. Every session listening on that channel received it, so
// any short-lived rela-postgres command (init, migrate, analyze) silently
// stopped rela-server's listener: jobs were still enqueued and announced, but
// no worker ever received them again. On Atlas that stopped every scheduled
// task until the next restart. Fixed in the neoq fork pinned by go.mod.
func TestPostgresQueue_SurvivesAnotherProcessClosing(t *testing.T) {
	admin := os.Getenv("RELA_TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("RELA_TEST_DATABASE_URL not set; skipping cross-process close case")
	}
	ctx := context.Background()
	dsn := schemaPinnedDSN(t, admin, "rela_jobs_otherclose_test")
	kind := jobs.NewKind("jobstest", "otherclose")

	server, err := jobs.NewPostgresQueue(ctx, discardLogger(), dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = server.Close(context.Background()) })
	var ran atomic.Int32
	require.NoError(t, server.Register(kind, func(context.Context, jobs.Job) error {
		ran.Add(1)
		return nil
	}))
	require.NoError(t, server.Start(ctx))

	require.NoError(t, server.Enqueue(ctx, jobs.Job{Kind: kind, Retry: jobs.RetryNever}))
	require.Eventually(t, func() bool { return ran.Load() == 1 }, 10*time.Second, 50*time.Millisecond)

	// What every rela-postgres CLI command does: build, start, close.
	cli, err := jobs.NewPostgresQueue(ctx, discardLogger(), dsn)
	require.NoError(t, err)
	require.NoError(t, cli.Start(ctx))
	require.NoError(t, cli.Close(ctx))

	require.NoError(t, server.Enqueue(ctx, jobs.Job{Kind: kind, Retry: jobs.RetryNever}))
	require.Eventually(t, func() bool { return ran.Load() == 2 }, 10*time.Second, 50*time.Millisecond,
		"the queue must keep processing after another process closes its queue")
}
