//go:build postgres

package jobs_test

import (
	"context"
	"os"
	"testing"

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
