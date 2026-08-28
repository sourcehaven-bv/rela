// Package schedulerstatetest is the shared contract every
// [schedulerstate.Store] implementation must satisfy.
//
// Two backends without a shared contract test is two subtly different
// behaviors, and the one that diverges will be the one nobody runs locally.
// One bug of exactly that shape already shipped in this codebase (a NUL byte in
// a job fingerprint that Go and the memory backend accept and PostgreSQL
// rejects), which is why the postgres arm is wired into CI and not only into a
// local recipe.
package schedulerstatetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
)

// NewStore builds a fresh, empty store for one subtest.
type NewStore func(t *testing.T) schedulerstate.Store

// RunAll runs the whole contract against newStore.
func RunAll(t *testing.T, newStore NewStore) {
	t.Helper()

	cases := []struct {
		name string
		run  func(*testing.T, NewStore)
	}{
		{"LoadMissingIsAbsentNotZero", testLoadMissingIsAbsentNotZero},
		{"RecordSuccessRoundTrips", testRecordSuccessRoundTrips},
		{"SuccessClearsTheLadder", testSuccessClearsTheLadder},
		{"WritesAreIsolatedPerTask", testWritesAreIsolatedPerTask},
		{"SuccessDoesNotRegress", testSuccessDoesNotRegress},
		{"FailureIncrementsAtomically", testFailureIncrementsAtomically},
		{"FailureCannotClobberANewerSuccess", testFailureCannotClobberANewerSuccess},
		{"SetNextRetryRoundTrips", testSetNextRetryRoundTrips},
		{"LoadIsScopedToNamedTasks", testLoadIsScopedToNamedTasks},
		{"PruneDropsOnlyStaleRecords", testPruneDropsOnlyStaleRecords},
		{"EmptyTaskRejected", testEmptyTaskRejected},
		{"ClosedStoreRejectsEverything", testClosedStoreRejectsEverything},
		{"ConcurrentWritersLoseNothing", testConcurrentWritersLoseNothing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newStore) })
	}
}

// base is a fixed instant. Every case derives from it rather than reading the
// clock, so ordering rules are pinned deterministically.
var base = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// retryDelay is a stand-in for a ladder step. The store never interprets
// it, so its value only has to be non-zero and stable.
const retryDelay = 5 * time.Minute

func testLoadMissingIsAbsentNotZero(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	got, err := s.Load(context.Background(), []string{"never-ran"})
	require.NoError(t, err)
	require.NotContains(t, got, "never-ran",
		"a task with no record must be ABSENT, so a caller can tell "+
			"'never ran' from 'ran at the zero time'")
}

func testRecordSuccessRoundTrips(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	ctx := context.Background()

	require.NoError(t, s.RecordSuccess(ctx, "task", base))

	got, err := s.Load(ctx, []string{"task"})
	require.NoError(t, err)
	require.True(t, got["task"].LastRun.Equal(base),
		"want %s, got %s", base, got["task"].LastRun)
}

// testSuccessClearsTheLadder pins the property whose absence re-opens
// BUG-ZKK2UL: a pending retry suppresses the ordinary schedule, so a success
// that left one set would pin the task to the ladder forever.
func testSuccessClearsTheLadder(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	ctx := context.Background()

	_, err := s.RecordFailure(ctx, "task", base)
	require.NoError(t, err)
	require.NoError(t, s.SetNextRetry(ctx, "task", base, base.Add(retryDelay)))

	require.NoError(t, s.RecordSuccess(ctx, "task", base.Add(time.Minute)))

	got, err := s.Load(ctx, []string{"task"})
	require.NoError(t, err)
	require.Zero(t, got["task"].Failures, "success must reset the failure count")
	require.True(t, got["task"].NextRetry.IsZero(),
		"success must clear the pending retry, or the task stays ladder-driven forever")
}

func testWritesAreIsolatedPerTask(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	ctx := context.Background()

	require.NoError(t, s.RecordSuccess(ctx, "a", base))
	require.NoError(t, s.RecordSuccess(ctx, "b", base))

	// Move only a.
	require.NoError(t, s.RecordSuccess(ctx, "a", base.Add(time.Hour)))

	got, err := s.Load(ctx, []string{"a", "b"})
	require.NoError(t, err)
	require.True(t, got["a"].LastRun.Equal(base.Add(time.Hour)))
	require.True(t, got["b"].LastRun.Equal(base),
		"a write about one task must not disturb another — the whole point of per-task records")
}

func testSuccessDoesNotRegress(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	ctx := context.Background()

	newer := base.Add(time.Hour)
	require.NoError(t, s.RecordSuccess(ctx, "task", newer))
	// A slow node reports an OLDER run afterwards.
	require.NoError(t, s.RecordSuccess(ctx, "task", base))

	got, err := s.Load(ctx, []string{"task"})
	require.NoError(t, err)
	require.True(t, got["task"].LastRun.Equal(newer),
		"a stale writer must not regress a newer last-run")
}

func testFailureIncrementsAtomically(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	ctx := context.Background()

	for want := 1; want <= 3; want++ {
		got, err := s.RecordFailure(ctx, "task", base.Add(time.Duration(want)*time.Minute))
		require.NoError(t, err)
		require.Equal(t, want, got, "the store owns the increment")
	}

	loaded, err := s.Load(ctx, []string{"task"})
	require.NoError(t, err)
	require.Equal(t, 3, loaded["task"].Failures)
}

// testFailureCannotClobberANewerSuccess pins the asymmetry that a first draft
// of this interface missed: node A succeeds, then node B — whose attempt
// STARTED earlier — reports its failure. Without a guard the healthy task is
// pinned to a retry ladder.
func testFailureCannotClobberANewerSuccess(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	ctx := context.Background()

	success := base.Add(time.Hour)
	require.NoError(t, s.RecordSuccess(ctx, "task", success))

	// B's run started before the success landed.
	failures, err := s.RecordFailure(ctx, "task", base)
	require.NoError(t, err)
	require.Zero(t, failures, "a stale failure must report the stored count, not resurrect a ladder")

	require.NoError(t, s.SetNextRetry(ctx, "task", base, base.Add(retryDelay)))

	got, err := s.Load(ctx, []string{"task"})
	require.NoError(t, err)
	require.Zero(t, got["task"].Failures)
	require.True(t, got["task"].NextRetry.IsZero(),
		"a task that has since succeeded must not be dragged back onto the ladder")
	require.True(t, got["task"].LastRun.Equal(success))
}

func testSetNextRetryRoundTrips(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	ctx := context.Background()

	_, err := s.RecordFailure(ctx, "task", base)
	require.NoError(t, err)

	retryAt := base.Add(retryDelay)
	require.NoError(t, s.SetNextRetry(ctx, "task", base, retryAt))

	got, err := s.Load(ctx, []string{"task"})
	require.NoError(t, err)
	require.True(t, got["task"].NextRetry.Equal(retryAt))
	require.True(t, got["task"].Failing())
}

func testLoadIsScopedToNamedTasks(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	ctx := context.Background()

	require.NoError(t, s.RecordSuccess(ctx, "wanted", base))
	require.NoError(t, s.RecordSuccess(ctx, "other-deployment", base))

	got, err := s.Load(ctx, []string{"wanted"})
	require.NoError(t, err)
	require.Contains(t, got, "wanted")
	require.NotContains(t, got, "other-deployment",
		"Load must not return rows the caller did not ask for")

	// An empty request is not an implicit "everything".
	all, err := s.Load(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, all)
}

func testPruneDropsOnlyStaleRecords(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	ctx := context.Background()

	old := base.Add(-48 * time.Hour)
	require.NoError(t, s.RecordSuccess(ctx, "stale", old))
	require.NoError(t, s.RecordSuccess(ctx, "fresh", base))

	removed, err := s.Prune(ctx, base.Add(-time.Hour))
	require.NoError(t, err)
	require.Equal(t, []string{"stale"}, removed)

	got, err := s.Load(ctx, []string{"stale", "fresh"})
	require.NoError(t, err)
	require.NotContains(t, got, "stale")
	require.Contains(t, got, "fresh")
}

func testEmptyTaskRejected(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	ctx := context.Background()

	require.ErrorIs(t, s.RecordSuccess(ctx, "", base), schedulerstate.ErrNoTask)
	_, err := s.RecordFailure(ctx, "", base)
	require.ErrorIs(t, err, schedulerstate.ErrNoTask)
	require.ErrorIs(t, s.SetNextRetry(ctx, "", base, base), schedulerstate.ErrNoTask)
}

func testClosedStoreRejectsEverything(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	ctx := context.Background()

	require.NoError(t, s.Close())
	require.NoError(t, s.Close(), "Close is idempotent")

	_, err := s.Load(ctx, []string{"task"})
	require.ErrorIs(t, err, schedulerstate.ErrClosed)
	require.ErrorIs(t, s.RecordSuccess(ctx, "task", base), schedulerstate.ErrClosed)
	_, err = s.RecordFailure(ctx, "task", base)
	require.ErrorIs(t, err, schedulerstate.ErrClosed)
	require.ErrorIs(t, s.SetNextRetry(ctx, "task", base, base), schedulerstate.ErrClosed)
	_, err = s.Prune(ctx, base)
	require.ErrorIs(t, err, schedulerstate.ErrClosed)
}

// testConcurrentWritersLoseNothing is the case that fails against the old
// whole-document storage: every writer there carried a full snapshot, so the
// last write won and the rest vanished.
func testConcurrentWritersLoseNothing(t *testing.T, newStore NewStore) {
	t.Helper()

	s := newStore(t)
	ctx := context.Background()

	const writers = 16
	var wg sync.WaitGroup
	errs := make([]error, writers)
	wg.Add(writers)
	for i := range writers {
		go func() {
			defer wg.Done()
			errs[i] = s.RecordSuccess(ctx, fmt.Sprintf("task-%02d", i), base)
		}()
	}
	wg.Wait()
	require.NoError(t, errors.Join(errs...))

	names := make([]string, writers)
	for i := range writers {
		names[i] = fmt.Sprintf("task-%02d", i)
	}
	got, err := s.Load(ctx, names)
	require.NoError(t, err)
	require.Len(t, got, writers, "every concurrent write must survive")
}
