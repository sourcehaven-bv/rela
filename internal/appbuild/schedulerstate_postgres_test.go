//go:build postgres

package appbuild_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/schedulerstate/pgschedstate"
)

// TestSchedulerState_PostgresBackend pins BUG-TKL08E's wiring: the postgres
// build's job queue is durable and shared, so a run queued by one process may
// execute on another, and its record must be in the database both can see.
func TestSchedulerState_PostgresBackend(t *testing.T) {
	svc, err := discover(t, writeMinimalProject(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	_, ok := svc.SchedulerState().(*pgschedstate.Store)
	require.True(t, ok, "SchedulerState = %T, want the postgres backend", svc.SchedulerState())
}
