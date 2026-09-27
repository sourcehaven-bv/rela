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
	"maps"
	"slices"
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
		run  func(*testing.T, schedulerstate.Store)
	}{
		{"LoadMissingIsAbsentNotZero", testLoadMissingIsAbsentNotZero},
		{"CreatedRunIsActive", testCreatedRunIsActive},
		{"OneActiveRunPerTask", testOneActiveRunPerTask},
		{"CreateRunRejectsStaleVersion", testCreateRunRejectsStaleVersion},
		{"StartRunOnlyOnce", testStartRunOnlyOnce},
		{"SuccessStampsCreatedAtAndClearsLadder", testSuccessStampsCreatedAtAndClearsLadder},
		{"FailureAdvancesLadder", testFailureAdvancesLadder},
		{"FinishRunIsIdempotent", testFinishRunIsIdempotent},
		{"QueuedRunCanFinish", testQueuedRunCanFinish},
		{"ReapAbandonsOnlyExpiredRuns", testReapAbandonsOnlyExpiredRuns},
		{"LateFailureAfterReapIsDiscarded", testLateFailureAfterReapIsDiscarded},
		{"LateSuccessAfterReapStampsLastRun", testLateSuccessAfterReapStampsLastRun},
		{"StartChildClaimsAndExtendsLease", testStartChildClaimsAndExtendsLease},
		{"StartChildRefusesSettledSubjectOrEndedRun", testStartChildRefusesSettledSubjectOrEndedRun},
		{"ExpectChildrenIsIdempotent", testExpectChildrenIsIdempotent},
		{"SettleChildOfAbandonedRunDoesNotRevive", testSettleChildOfAbandonedRunDoesNotRevive},
		{"ChildrenSettleOnceAndFinishTheRun", testChildrenSettleOnceAndFinishTheRun},
		{"ChildrenAllSucceeding", testChildrenAllSucceeding},
		{"SucceededSubjectsSpanRunsOfOneOccurrence", testSucceededSubjectsSpanRunsOfOneOccurrence},
		{"WritesAreIsolatedPerTask", testWritesAreIsolatedPerTask},
		{"SeedNeverOverwrites", testSeedNeverOverwrites},
		{"SeedWithoutTimesIsNoOp", testSeedWithoutTimesIsNoOp},
		{"LoadIsScopedToNamedTasks", testLoadIsScopedToNamedTasks},
		{"PruneDropsOnlyStaleRecords", testPruneDropsOnlyStaleRecords},
		{"InvalidArgumentsRejected", testInvalidArgumentsRejected},
		{"ClosedStoreRejectsEverything", testClosedStoreRejectsEverything},
		{"ConcurrentCreateAdmitsOne", testConcurrentCreateAdmitsOne},
		{"ConcurrentWritersLoseNothing", testConcurrentWritersLoseNothing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newStore(t)) })
	}
}

// base is a fixed instant. Every case derives from it rather than reading the
// clock, so ordering rules are pinned deterministically.
var base = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

const (
	leaseWindow = 30 * time.Minute
	day         = 24 * time.Hour
)

// lease is a stand-in lease length. The store never interprets it.
const lease = 20 * time.Minute

// ladder is a stand-in retry policy: n minutes after the nth failure.
func ladder(failures int) time.Duration { return time.Duration(failures) * time.Minute }

// newRun returns a queued run of task created at base.
func newRun(task, id string) schedulerstate.Run {
	return schedulerstate.Run{ID: id, Task: task, CreatedAt: base, LeaseUntil: base.Add(lease)}
}

// load reads one task's state, failing the test when it is absent.
func load(t *testing.T, s schedulerstate.Store, task string) schedulerstate.TaskState {
	t.Helper()
	got, err := s.Load(context.Background(), []string{task})
	require.NoError(t, err)
	ts, ok := got[task]
	require.True(t, ok, "task %q has no record", task)
	return ts
}

// runToEnd creates, starts and finishes a run of task with out.
func runToEnd(
	t *testing.T, s schedulerstate.Store, task, id string, created time.Time, out schedulerstate.Outcome,
) schedulerstate.Finished {
	t.Helper()
	ctx := context.Background()

	version := int64(0)
	if got, err := s.Load(ctx, []string{task}); err == nil {
		version = got[task].Version
	}
	run := newRun(task, id)
	run.CreatedAt = created
	require.NoError(t, s.CreateRun(ctx, run, version))
	_, err := s.StartRun(ctx, id, "node-a", created, created.Add(lease))
	require.NoError(t, err)
	fin, err := s.FinishRun(ctx, id, out, ladder)
	require.NoError(t, err)
	return fin
}

func testLoadMissingIsAbsentNotZero(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	got, err := s.Load(context.Background(), []string{"never-ran"})
	require.NoError(t, err)
	require.NotContains(t, got, "never-ran",
		"a task with no record must be ABSENT, so a caller can tell "+
			"'never ran' from 'ran at the zero time'")
}

func testCreatedRunIsActive(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	require.NoError(t, s.CreateRun(context.Background(), newRun("task", "r1"), 0))

	ts := load(t, s, "task")
	require.NotNil(t, ts.Active)
	require.Equal(t, "r1", ts.Active.ID)
	require.Equal(t, schedulerstate.RunQueued, ts.Active.Status)
	require.True(t, ts.LastRun.IsZero(), "creating a run must not count as having run")
}

func testOneActiveRunPerTask(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, s.CreateRun(ctx, newRun("task", "r1"), 0))

	require.ErrorIs(t, s.CreateRun(ctx, newRun("task", "r2"), 0), schedulerstate.ErrRunActive,
		"a task whose run is still queued must not get a second one")

	_, err := s.StartRun(ctx, "r1", "node-a", base, base.Add(lease))
	require.NoError(t, err)
	require.ErrorIs(t, s.CreateRun(ctx, newRun("task", "r2"), 0), schedulerstate.ErrRunActive,
		"nor while it is running")

	require.NoError(t, s.CreateRun(ctx, newRun("other", "r3"), 0),
		"the rule is per task: another task is unaffected")
}

func testCreateRunRejectsStaleVersion(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	runToEnd(t, s, "task", "r1", base, schedulerstate.Outcome{At: base.Add(time.Minute)})

	require.ErrorIs(t, s.CreateRun(ctx, newRun("task", "r2"), 0), schedulerstate.ErrStale,
		"a caller that loaded the task before the last outcome decided on old state")
	require.NoError(t, s.CreateRun(ctx, newRun("task", "r2"), load(t, s, "task").Version))
}

func testStartRunOnlyOnce(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, s.CreateRun(ctx, newRun("task", "r1"), 0))

	started, err := s.StartRun(ctx, "r1", "node-a", base.Add(time.Second), base.Add(lease))
	require.NoError(t, err)
	require.Equal(t, schedulerstate.RunRunning, started.Status)
	require.Equal(t, "node-a", started.Node)
	require.True(t, started.StartedAt.Equal(base.Add(time.Second)))

	again, err := s.StartRun(ctx, "r1", "node-b", base.Add(2*time.Second), base.Add(lease))
	require.ErrorIs(t, err, schedulerstate.ErrNotQueued,
		"a redelivered job must learn it is a duplicate rather than run again")
	require.Equal(t, "node-a", again.Node, "the duplicate is told who owns the run")

	_, err = s.StartRun(ctx, "missing", "node-a", base, base.Add(lease))
	require.ErrorIs(t, err, schedulerstate.ErrNoRun)
}

func testSuccessStampsCreatedAtAndClearsLadder(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	runToEnd(t, s, "task", "r1", base, schedulerstate.Outcome{Error: "boom", At: base.Add(time.Minute)})
	require.True(t, load(t, s, "task").Failing())

	created := base.Add(time.Hour)
	fin := runToEnd(t, s, "task", "r2", created, schedulerstate.Outcome{At: created.Add(10 * time.Minute)})
	require.True(t, fin.Applied)
	require.Equal(t, schedulerstate.RunSucceeded, fin.Run.Status)

	ts := load(t, s, "task")
	require.True(t, ts.LastRun.Equal(created),
		"LastRun is the run's START, so a long run does not drift the schedule")
	require.Zero(t, ts.Failures, "success is the only reset")
	require.True(t, ts.NextRetry.IsZero())
	require.Nil(t, ts.Active)
}

func testFailureAdvancesLadder(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	first := runToEnd(t, s, "task", "r1", base, schedulerstate.Outcome{Error: "boom", At: base.Add(time.Minute)})
	require.Equal(t, schedulerstate.RunFailed, first.Run.Status)
	require.Equal(t, "boom", first.Run.Error)
	require.Equal(t, 1, first.Failures)
	require.True(t, first.NextRetry.Equal(base.Add(2*time.Minute)),
		"the retry is measured from when the failure was observed")

	observed := base.Add(time.Hour)
	second := runToEnd(t, s, "task", "r2", base.Add(3*time.Minute),
		schedulerstate.Outcome{Error: "boom", At: observed})
	require.Equal(t, 2, second.Failures)

	ts := load(t, s, "task")
	require.Equal(t, 2, ts.Failures)
	require.True(t, ts.NextRetry.Equal(observed.Add(2*time.Minute)))
	require.True(t, ts.LastRun.IsZero(), "a failure must not count as having run")
}

func testFinishRunIsIdempotent(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	runToEnd(t, s, "task", "r1", base, schedulerstate.Outcome{Error: "boom", At: base.Add(time.Minute)})

	again, err := s.FinishRun(ctx, "r1", schedulerstate.Outcome{Error: "boom", At: base.Add(2 * time.Minute)}, ladder)
	require.NoError(t, err)
	require.False(t, again.Applied)
	require.Equal(t, 1, load(t, s, "task").Failures, "a repeated finish must not count twice")
}

func testQueuedRunCanFinish(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, s.CreateRun(ctx, newRun("task", "r1"), 0))

	// The job never reached the queue: the scheduler fails the run directly.
	fin, err := s.FinishRun(ctx, "r1", schedulerstate.Outcome{Error: "enqueue failed", At: base}, ladder)
	require.NoError(t, err)
	require.True(t, fin.Applied)
	require.Equal(t, schedulerstate.RunFailed, fin.Run.Status)
	require.Nil(t, load(t, s, "task").Active)
}

func testReapAbandonsOnlyExpiredRuns(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()

	queued := newRun("queued", "r1")
	queued.LeaseUntil = base.Add(time.Minute)
	require.NoError(t, s.CreateRun(ctx, queued, 0))

	require.NoError(t, s.CreateRun(ctx, newRun("running", "r2"), 0))
	_, err := s.StartRun(ctx, "r2", "node-a", base, base.Add(time.Minute))
	require.NoError(t, err)

	require.NoError(t, s.CreateRun(ctx, newRun("healthy", "r3"), 0))
	_, err = s.StartRun(ctx, "r3", "node-a", base, base.Add(time.Hour))
	require.NoError(t, err)

	now := base.Add(10 * time.Minute)
	reaped, err := s.Reap(ctx, now, ladder)
	require.NoError(t, err)
	require.Len(t, reaped, 2)
	for _, fin := range reaped {
		require.Equal(t, schedulerstate.RunAbandoned, fin.Run.Status)
		require.NotEmpty(t, fin.Run.Error)
		require.Equal(t, 1, fin.Failures, "an abandoned run counts as a failure")
		require.True(t, fin.NextRetry.Equal(now.Add(time.Minute)))
	}

	require.Nil(t, load(t, s, "queued").Active, "the task is free for its next run")
	require.NotNil(t, load(t, s, "healthy").Active, "a run within its lease is left alone")

	again, err := s.Reap(ctx, now, ladder)
	require.NoError(t, err)
	require.Empty(t, again, "reaping is idempotent")
}

// reapedRun leaves run r1 of task abandoned after one failure.
func reapedRun(t *testing.T, s schedulerstate.Store, task string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, s.CreateRun(ctx, newRun(task, "r1"), 0))
	_, err := s.StartRun(ctx, "r1", "node-a", base, base.Add(time.Minute))
	require.NoError(t, err)
	reaped, err := s.Reap(ctx, base.Add(time.Hour), ladder)
	require.NoError(t, err)
	require.Len(t, reaped, 1)
}

func testLateFailureAfterReapIsDiscarded(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	reapedRun(t, s, "task")

	late, err := s.FinishRun(context.Background(), "r1",
		schedulerstate.Outcome{Error: "boom", At: base.Add(2 * time.Hour)}, ladder)
	require.NoError(t, err)
	require.False(t, late.Applied, "a run already abandoned cannot be revived by its late worker")
	require.False(t, late.LateSuccess)
	require.Equal(t, schedulerstate.RunAbandoned, late.Run.Status)
	require.Equal(t, 1, load(t, s, "task").Failures, "the abandonment already counted this failure")
}

// testLateSuccessAfterReapStampsLastRun pins that work which did happen is not
// run again: the run stays abandoned, but the task records it as done.
func testLateSuccessAfterReapStampsLastRun(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	reapedRun(t, s, "task")

	late, err := s.FinishRun(context.Background(), "r1", schedulerstate.Outcome{At: base.Add(2 * time.Hour)}, ladder)
	require.NoError(t, err)
	require.False(t, late.Applied)
	require.True(t, late.LateSuccess)
	require.Equal(t, schedulerstate.RunAbandoned, late.Run.Status)

	ts := load(t, s, "task")
	require.True(t, ts.LastRun.Equal(base), "LastRun is the run's creation time")
	require.Zero(t, ts.Failures)
	require.True(t, ts.NextRetry.IsZero())
}

// fanOutRun creates, starts and fans out run r1 of task "fan" to subjects.
func fanOutRun(t *testing.T, s schedulerstate.Store, subjects ...string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, s.CreateRun(ctx, newRun("fan", "r1"), 0))
	_, err := s.StartRun(ctx, "r1", "node-a", base, base.Add(time.Minute))
	require.NoError(t, err)
	require.NoError(t, s.ExpectChildren(ctx, "r1", subjects))
}

func testStartChildClaimsAndExtendsLease(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	fanOutRun(t, s, "a", "b")

	claimed, err := s.StartChild(ctx, "r1", "a", base.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, claimed)
	claimed, err = s.StartChild(ctx, "r1", "a", base.Add(2*time.Minute))
	require.NoError(t, err)
	require.True(t, claimed, "a subject stays claimable until it settles, for the queue's retries")
	require.True(t, load(t, s, "fan").Active.LeaseUntil.Equal(base.Add(time.Hour)), "a lease never moves backwards")

	reaped, err := s.Reap(ctx, base.Add(leaseWindow), ladder)
	require.NoError(t, err)
	require.Empty(t, reaped, "an extended lease protects the run")

	_, err = s.StartChild(ctx, "r1", "unknown", base.Add(time.Hour))
	require.Error(t, err, "a subject the run never expected is rejected")
	_, err = s.StartChild(ctx, "missing", "a", base.Add(time.Hour))
	require.ErrorIs(t, err, schedulerstate.ErrNoRun)
}

// testStartChildRefusesSettledSubjectOrEndedRun pins the guard against a
// repeated side effect: a redelivered child, and a child of a run a retry has
// replaced, must not execute.
func testStartChildRefusesSettledSubjectOrEndedRun(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	fanOutRun(t, s, "a", "b")

	_, _, err := s.SettleChild(ctx, "r1", "a", schedulerstate.Outcome{At: base}, ladder)
	require.NoError(t, err)
	claimed, err := s.StartChild(ctx, "r1", "a", base.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, claimed, "a settled subject is not executed again")

	_, err = s.Reap(ctx, base.Add(time.Hour), ladder)
	require.NoError(t, err)
	claimed, err = s.StartChild(ctx, "r1", "b", base.Add(2*time.Hour))
	require.NoError(t, err)
	require.False(t, claimed, "a child of an abandoned run is not executed")
}

// testExpectChildrenIsIdempotent covers a redelivered expansion: recording the
// same subjects again must not inflate the count the run waits for.
func testExpectChildrenIsIdempotent(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	fanOutRun(t, s, "a", "b")
	require.NoError(t, s.ExpectChildren(ctx, "r1", []string{"b", "a"}))
	require.Equal(t, 2, load(t, s, "fan").Active.Children)

	for _, subject := range []string{"a", "b"} {
		_, _, err := s.SettleChild(ctx, "r1", subject, schedulerstate.Outcome{At: base}, ladder)
		require.NoError(t, err)
	}
	require.Nil(t, load(t, s, "fan").Active, "the run ends once both subjects settle")
}

func testSettleChildOfAbandonedRunDoesNotRevive(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	fanOutRun(t, s, "a")
	_, err := s.Reap(ctx, base.Add(time.Hour), ladder)
	require.NoError(t, err)

	settled, done, err := s.SettleChild(ctx, "r1", "a", schedulerstate.Outcome{At: base.Add(time.Hour)}, ladder)
	require.NoError(t, err)
	require.True(t, settled, "the subject's own outcome is still recorded")
	require.Nil(t, done, "an abandoned run does not end a second time")
	require.Equal(t, 1, load(t, s, "fan").Failures)
}

func testChildrenSettleOnceAndFinishTheRun(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, s.CreateRun(ctx, newRun("fan", "r1"), 0))
	_, err := s.StartRun(ctx, "r1", "node-a", base, base.Add(lease))
	require.NoError(t, err)
	require.NoError(t, s.ExpectChildren(ctx, "r1", []string{"a", "b", "c"}))

	at := base.Add(time.Minute)
	settled, done, err := s.SettleChild(ctx, "r1", "a", schedulerstate.Outcome{Error: "smtp down", At: at}, ladder)
	require.NoError(t, err)
	require.True(t, settled)
	require.Nil(t, done)

	settled, done, err = s.SettleChild(ctx, "r1", "a", schedulerstate.Outcome{At: at}, ladder)
	require.NoError(t, err)
	require.False(t, settled, "a redelivered child must not settle twice")
	require.Nil(t, done)

	_, done, err = s.SettleChild(ctx, "r1", "b", schedulerstate.Outcome{At: at}, ladder)
	require.NoError(t, err)
	require.Nil(t, done)
	require.NotNil(t, load(t, s, "fan").Active, "the run is active until every subject settles")

	_, done, err = s.SettleChild(ctx, "r1", "c", schedulerstate.Outcome{At: at.Add(time.Minute)}, ladder)
	require.NoError(t, err)
	require.NotNil(t, done, "the last subject ends the run")
	require.Equal(t, schedulerstate.RunFailed, done.Run.Status, "any failed subject fails the run (BUG-1YMHIS)")
	require.Equal(t, "1 of 3 for_each subjects failed; first: smtp down", done.Run.Error)
	require.Equal(t, 1, done.Failures)

	_, _, err = s.SettleChild(ctx, "r1", "unknown", schedulerstate.Outcome{At: at}, ladder)
	require.Error(t, err, "a subject the run never expected is rejected")
}

func testChildrenAllSucceeding(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, s.CreateRun(ctx, newRun("fan", "r1"), 0))
	_, err := s.StartRun(ctx, "r1", "node-a", base, base.Add(lease))
	require.NoError(t, err)
	require.NoError(t, s.ExpectChildren(ctx, "r1", []string{"a", "b"}))

	for _, subject := range []string{"b", "a"} {
		_, _, err = s.SettleChild(ctx, "r1", subject, schedulerstate.Outcome{At: base.Add(time.Minute)}, ladder)
		require.NoError(t, err)
	}
	ts := load(t, s, "fan")
	require.Nil(t, ts.Active)
	require.True(t, ts.LastRun.Equal(base))
}

// testSucceededSubjectsSpanRunsOfOneOccurrence pins what a retried fan-out
// relies on to not repeat a delivered subject: successes from every run of the
// occurrence count, failures and other occurrences do not.
func testSucceededSubjectsSpanRunsOfOneOccurrence(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	fanOut := func(id, occurrence string, version int64, outcomes map[string]string) {
		t.Helper()
		run := newRun("fan", id)
		run.Occurrence = occurrence
		require.NoError(t, s.CreateRun(ctx, run, version))
		_, err := s.StartRun(ctx, id, "node-a", base, base.Add(lease))
		require.NoError(t, err)
		subjects := slices.Sorted(maps.Keys(outcomes))
		require.NoError(t, s.ExpectChildren(ctx, id, subjects))
		for _, subject := range subjects {
			_, _, err := s.SettleChild(ctx, id, subject,
				schedulerstate.Outcome{Error: outcomes[subject], At: base.Add(time.Minute)}, ladder)
			require.NoError(t, err)
		}
	}

	fanOut("r1", "2026-03-01", 0, map[string]string{"a": "", "b": "smtp down", "c": ""})
	fanOut("r2", "2026-03-01", load(t, s, "fan").Version, map[string]string{"b": "smtp down"})
	fanOut("r3", "2026-02-28", load(t, s, "fan").Version, map[string]string{"d": ""})

	got, err := s.SucceededSubjects(ctx, "fan", "2026-03-01")
	require.NoError(t, err)
	require.Equal(t, []string{"a", "c"}, got)

	got, err = s.SucceededSubjects(ctx, "other", "2026-03-01")
	require.NoError(t, err)
	require.Empty(t, got)
}

func testWritesAreIsolatedPerTask(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	runToEnd(t, s, "a", "r1", base, schedulerstate.Outcome{At: base.Add(time.Minute)})
	runToEnd(t, s, "b", "r2", base, schedulerstate.Outcome{Error: "boom", At: base.Add(time.Minute)})

	a := load(t, s, "a")
	require.True(t, a.LastRun.Equal(base))
	require.Zero(t, a.Failures, "b's failure must not leak into a")
	require.Equal(t, 1, load(t, s, "b").Failures)
}

func testSeedNeverOverwrites(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	legacy := schedulerstate.TaskState{LastRun: base, Failures: 2, NextRetry: base.Add(time.Hour)}
	require.NoError(t, s.Seed(ctx, "imported", legacy))

	got := load(t, s, "imported")
	require.True(t, got.LastRun.Equal(base))
	require.Equal(t, 2, got.Failures)
	require.True(t, got.NextRetry.Equal(base.Add(time.Hour)))

	runToEnd(t, s, "task", "r1", base.Add(time.Hour), schedulerstate.Outcome{At: base.Add(2 * time.Hour)})
	require.NoError(t, s.Seed(ctx, "task", legacy))
	require.True(t, load(t, s, "task").LastRun.Equal(base.Add(time.Hour)),
		"an import must never overwrite newer state")
}

func testSeedWithoutTimesIsNoOp(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	require.NoError(t, s.Seed(context.Background(), "empty", schedulerstate.TaskState{Failures: 1}))
	got, err := s.Load(context.Background(), []string{"empty"})
	require.NoError(t, err)
	require.NotContains(t, got, "empty", "a record with nothing to date it by is not stored")
}

func testLoadIsScopedToNamedTasks(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	runToEnd(t, s, "a", "r1", base, schedulerstate.Outcome{At: base})
	runToEnd(t, s, "b", "r2", base, schedulerstate.Outcome{At: base})

	got, err := s.Load(context.Background(), []string{"a"})
	require.NoError(t, err)
	require.Contains(t, got, "a")
	require.NotContains(t, got, "b")
}

func testPruneDropsOnlyStaleRecords(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	runToEnd(t, s, "old", "r1", base, schedulerstate.Outcome{At: base})
	runToEnd(t, s, "fresh", "r2", base.Add(2*day), schedulerstate.Outcome{At: base.Add(2 * day)})
	stuck := newRun("stuck", "r3")
	require.NoError(t, s.CreateRun(ctx, stuck, 0))

	removed, err := s.Prune(ctx, base.Add(day))
	require.NoError(t, err)
	require.Equal(t, []string{"old"}, removed)

	got, err := s.Load(ctx, []string{"old", "fresh", "stuck"})
	require.NoError(t, err)
	require.NotContains(t, got, "old")
	require.Contains(t, got, "fresh")
	require.Contains(t, got, "stuck", "a task with an active run is never pruned")

	_, err = s.FinishRun(ctx, "r1", schedulerstate.Outcome{At: base}, ladder)
	require.ErrorIs(t, err, schedulerstate.ErrNoRun, "ended runs older than the cut-off are dropped")
}

func testInvalidArgumentsRejected(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	require.ErrorIs(t, s.CreateRun(ctx, newRun("", "r1"), 0), schedulerstate.ErrNoTask)
	require.ErrorIs(t, s.CreateRun(ctx, newRun("task", ""), 0), schedulerstate.ErrNoRun)
	require.ErrorIs(t, s.Seed(ctx, "", schedulerstate.TaskState{}), schedulerstate.ErrNoTask)
	_, err := s.FinishRun(ctx, "missing", schedulerstate.Outcome{At: base}, ladder)
	require.ErrorIs(t, err, schedulerstate.ErrNoRun)
	require.ErrorIs(t, s.ExpectChildren(ctx, "missing", []string{"a"}), schedulerstate.ErrNoRun)
}

func testClosedStoreRejectsEverything(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, s.Close())
	require.NoError(t, s.Close(), "Close is idempotent")

	_, err := s.Load(ctx, []string{"task"})
	require.ErrorIs(t, err, schedulerstate.ErrClosed)
	require.ErrorIs(t, s.CreateRun(ctx, newRun("task", "r1"), 0), schedulerstate.ErrClosed)
	_, err = s.StartRun(ctx, "r1", "node-a", base, base)
	require.ErrorIs(t, err, schedulerstate.ErrClosed)
	_, err = s.FinishRun(ctx, "r1", schedulerstate.Outcome{At: base}, ladder)
	require.ErrorIs(t, err, schedulerstate.ErrClosed)
	_, err = s.Reap(ctx, base, ladder)
	require.ErrorIs(t, err, schedulerstate.ErrClosed)
	_, err = s.Prune(ctx, base)
	require.ErrorIs(t, err, schedulerstate.ErrClosed)
	_, err = s.SucceededSubjects(ctx, "task", "occ")
	require.ErrorIs(t, err, schedulerstate.ErrClosed)
	_, err = s.StartChild(ctx, "r1", "a", base)
	require.ErrorIs(t, err, schedulerstate.ErrClosed)
}

// testConcurrentCreateAdmitsOne is the cross-process guarantee in miniature:
// several schedulers that all decided a task is due create exactly one run.
func testConcurrentCreateAdmitsOne(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	const writers = 8
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		admitted int
		others   []error
	)
	for i := range writers {
		wg.Go(func() {
			err := s.CreateRun(context.Background(), newRun("task", fmt.Sprintf("r%d", i)), 0)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				admitted++
				return
			}
			others = append(others, err)
		})
	}
	wg.Wait()

	require.Equal(t, 1, admitted, "exactly one concurrent CreateRun may win")
	for _, err := range others {
		require.True(t, errors.Is(err, schedulerstate.ErrRunActive) || errors.Is(err, schedulerstate.ErrStale),
			"losers must be told why: %v", err)
	}
}

// testConcurrentWritersLoseNothing pins the property the per-task design
// exists for: concurrent outcomes for different tasks never clobber each other.
func testConcurrentWritersLoseNothing(t *testing.T, s schedulerstate.Store) {
	t.Helper()
	const tasks = 8
	var wg sync.WaitGroup
	for i := range tasks {
		wg.Go(func() {
			name := fmt.Sprintf("task-%d", i)
			runToEnd(t, s, name, "run-"+name, base, schedulerstate.Outcome{Error: "boom", At: base})
		})
	}
	wg.Wait()

	for i := range tasks {
		require.Equal(t, 1, load(t, s, fmt.Sprintf("task-%d", i)).Failures)
	}
}
