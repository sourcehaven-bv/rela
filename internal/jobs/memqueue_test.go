package jobs_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/jobs/jobstest"
)

// discardLogger keeps conformance output readable — these tests deliberately
// provoke handler failures, and the resulting warn lines are expected.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestMemoryQueue_Conformance(t *testing.T) {
	jobstest.RunAll(t, func(t *testing.T) jobs.Queue {
		t.Helper()
		q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
		require.NoError(t, err)
		t.Cleanup(func() { _ = q.Close(context.Background()) })
		return q
	})
}

func TestNewMemoryQueue_RejectsNilLogger(t *testing.T) {
	// Constructors reject nil required collaborators rather than substituting
	// a no-op: a silent no-op logger would hide every job failure.
	_, err := jobs.NewMemoryQueue(context.Background(), nil)
	require.Error(t, err)
}
