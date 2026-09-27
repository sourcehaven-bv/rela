package appbuild_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/cli"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// This file covers the one segment nothing else does: a real Lua script,
// executed by a real engine, on a real job-queue worker, mutating a real store.
//
// Everything in internal/scheduler stubs the engine via engineRunner, so the
// chain those tests cover stops at the handler:
//
//	scheduler → queue → worker → [stub]
//
// That gap is not theoretical. Running the full chain by hand found three
// defects a green suite had missed — a CLI entry point that never attached a
// queue, a NUL byte in the dedupe fingerprint that PostgreSQL rejects, and an
// attempt counter stored where postgres discards it. Two of the three lived
// past the stub. These tests exist so that segment is guarded by CI rather
// than by remembering to run a demo.
//
// They go through appbuild because that is where the real pieces meet:
// Services supplies both the job queue and the ACL-bound write deps a script
// needs to touch the graph.

// writeSchedulerProject creates a project whose single task runs the given Lua.
//
// The interval is fixed at 1h: these tests drive execution directly rather than
// waiting for a tick, so the cadence only has to be long enough not to fire
// twice on its own.
//
// The script is deliberately one that WRITES: a read-only script would still
// exercise the queue but would not prove that a job running on a worker can
// reach the store through the write path, which is the point.
func writeSchedulerProject(t *testing.T, luaSource string) string {
	t.Helper()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(root, "schema.yaml"), []byte(`
entities:
  note:
    label: Note
    id_type: short
    id_prefix: "NOTE-"
    properties:
      title: {type: string, required: true}
      runs: {type: string}
`), 0o600))

	require.NoError(t, os.WriteFile(filepath.Join(root, "schedules.yaml"),
		[]byte("tasks:\n  - name: tick\n    script: tick.lua\n    every: 1h\n"), 0o600))

	require.NoError(t, os.WriteFile(
		filepath.Join(root, "scripts", "tick.lua"), []byte(luaSource), 0o600))

	return root
}

// countingScript creates a note on first run and increments it thereafter, so
// the number of executions is readable from the graph itself.
const countingScript = `
local existing = rela.list_entities("note")
if #existing == 0 then
  rela.create_entity("note", { title = "heartbeat", runs = "1" })
else
  local n = existing[1]
  rela.update_entity(n.id, { runs = tostring(tonumber(n.properties.runs or "0") + 1) })
end
`

// TestScheduler_EndToEnd_LuaWritesThroughTheQueue is the full chain: a task
// becomes a job, a worker runs it, the script executes, and an entity lands in
// the store.
//
// No engineRunner stub, no fake queue. If any link is broken — the handler is
// not registered, the job is dropped, the write deps are not reachable from a
// worker goroutine — no entity appears and this fails.
func TestScheduler_EndToEnd_LuaWritesThroughTheQueue(t *testing.T) {
	root := writeSchedulerProject(t, countingScript)

	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	runScheduler(t, svc)

	notes := listNotes(t, svc)
	require.Len(t, notes, 1, "the scheduled script must have created an entity")
	require.Equal(t, "1", notes[0], "first run should record one execution")
}

// TestScheduler_EndToEnd_RepeatedRunsAccumulate proves a second run reaches the
// store too, and reads what the first one wrote.
//
// A job that ran once could pass the test above while a redelivery, a stale
// snapshot, or a dedupe key that never frees would break the second. Reading
// the previous value back is what makes this an integration test rather than
// two independent smoke tests.
func TestScheduler_EndToEnd_RepeatedRunsAccumulate(t *testing.T) {
	root := writeSchedulerProject(t, countingScript)

	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	// One scheduler, two runs. The task is forced due again by rewinding its
	// last-run stamp, which is what the scheduler itself reads — so the second
	// run goes through the same due-evaluation as a real tick would.
	//
	// A second NewWithQueue over the same Services would fail: the first
	// scheduler already registered TaskKind, and Register rejects a duplicate
	// kind. That is correct behavior (two schedulers sharing one queue would
	// both handle every task), and worth knowing — it means a Services carries
	// at most one scheduler.
	runSchedulerTwice(t, svc)

	notes := listNotes(t, svc)
	require.Len(t, notes, 1, "the script updates one entity; it must not create a second")
	require.Equal(t, "2", notes[0],
		"the second run must have read the first run's value and incremented it")
}

// TestScheduler_EndToEnd_FailingScriptAdvancesTheLadder pins that a genuine Lua
// error travels back from the worker and is recorded as a failure.
//
// The retry ladder is the scheduler's most load-bearing behavior (BUG-ZKK2UL),
// and it only works if the outcome of an asynchronous run reaches the state
// bookkeeping. A stubbed engine cannot prove a real script error does.
func TestScheduler_EndToEnd_FailingScriptAdvancesTheLadder(t *testing.T) {
	root := writeSchedulerProject(t, `error("deliberate failure")`)

	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	runScheduler(t, svc)

	ts := tickState(t, svc)
	require.Equal(t, 1, ts.Failures, "a failing script must advance the retry ladder")
	require.False(t, ts.NextRetry.IsZero(), "a failed task must have a pending retry (BUG-ZKK2UL)")
	require.True(t, ts.LastRun.IsZero(), "a failed run must not stamp a last-successful-run time")
}

// TestSchedulerCmd_EndToEnd drives the `rela scheduler` COMMAND, not the
// scheduler type.
//
// This is the test that would have caught the real regression: the command
// built its scheduler with New and never attached a queue, so after inline
// execution was removed it started cleanly, logged a due task, and failed every
// one with "no job queue configured". Every other test constructs a Scheduler
// itself and calls UseQueue, so none of them touched the wiring the command
// actually uses.
//
// Asserting on the graph rather than on the command's error is deliberate: the
// broken version returned no error at all. The only observable difference was
// that nothing happened.
func TestSchedulerCmd_EndToEnd(t *testing.T) {
	root := writeSchedulerProject(t, countingScript)

	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := &cli.SchedulerCmd{}
	done := make(chan error, 1)
	go func() { done <- cmd.Run(ctx, svc) }()

	require.Eventually(t, func() bool {
		notes := listNotes(t, svc)
		return len(notes) == 1 && notes[0] == "1"
	}, settleFor, 20*time.Millisecond,
		"the scheduler command did not execute its task; check that it attaches the job queue")

	// The note is what the task DOES; the run record is how the scheduler
	// records that it is DONE, and the worker writes it AFTER the note
	// exists. Returning on the note alone lets that write race t.TempDir's
	// RemoveAll, which then fails with ".rela: directory not empty"
	// (BUG-PRFSTS). Wait for the run's last write, not its first visible one.
	require.Eventually(t, func() bool { return schedulerSettled(svc) },
		settleFor, 20*time.Millisecond,
		"the scheduler did not persist its state; the run is not finished")

	cancel()
	select {
	case <-done:
	case <-time.After(settleFor):
		t.Fatal("scheduler command did not stop")
	}
}

// runScheduler builds a scheduler the way an entry point does, runs it until it
// has recorded a task outcome, then stops it.
//
// It uses NewWithQueue and Run — the production constructor and the production
// loop — rather than reaching into internals. That is deliberate: the CLI
// regression this suite exists to prevent was a wiring mistake at exactly this
// level, invisible to any test that constructed a Scheduler by hand.
func runScheduler(t *testing.T, svc *appbuild.Services) {
	t.Helper()

	data, err := svc.Config().Load(context.Background(), scheduler.ConfigFile)
	require.NoError(t, err)
	cfg, err := scheduler.ParseConfig(data)
	require.NoError(t, err)

	s, err := scheduler.NewWithQueue(cfg, script.NewEngine(), svc, discardTestLogger())
	require.NoError(t, err)

	// Run executes due tasks immediately on start, so the first run lands
	// without waiting for a tick. Stop as soon as the run has ended.
	runUntil(t, s, func() bool { return schedulerSettled(svc) },
		"the scheduler did not finish a task run")
}

// runUntil starts s, waits for settled to hold, then stops it.
//
// Extracted because every e2e case needs the same start/await/stop dance, and
// the stop half in particular (cancel, then confirm Run actually returned) is
// easy to get subtly wrong or omit.
func runUntil(t *testing.T, s *scheduler.Scheduler, settled func() bool, msg string) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	require.Eventually(t, settled, settleFor, 20*time.Millisecond, msg)

	cancel()
	select {
	case <-done:
	case <-time.After(settleFor):
		t.Fatal("scheduler did not stop")
	}
}

// runSchedulerTwice runs one scheduler through two task executions.
//
// Between them it forgets the task's run-state, so the task is due again as a
// first run, rather than waiting out a real interval.
func runSchedulerTwice(t *testing.T, svc *appbuild.Services) {
	t.Helper()

	data, err := svc.Config().Load(context.Background(), scheduler.ConfigFile)
	require.NoError(t, err)
	cfg, err := scheduler.ParseConfig(data)
	require.NoError(t, err)

	s, err := scheduler.NewWithQueue(cfg, script.NewEngine(), svc, discardTestLogger())
	require.NoError(t, err)

	runUntil(t, s, func() bool { return schedulerSettled(svc) },
		"first run did not complete")

	// Prune with a cut-off in the future drops every idle record, so the task
	// is due again on the same scheduler's first tick.
	forgotten := time.Now()
	_, err = svc.SchedulerState().Prune(context.Background(), forgotten.Add(time.Hour))
	require.NoError(t, err)

	// Both conditions, for the reason given in TestSchedulerCmd_EndToEnd: the
	// note proves the task ran, the run record proves the run is over.
	runUntil(t, s, func() bool {
		notes := listNotes(t, svc)
		return len(notes) > 0 && notes[0] == "2" && lastRunAfter(svc, forgotten)
	}, "second run did not complete")
}

// tickState returns the run-state of the "tick" task.
func tickState(t *testing.T, svc *appbuild.Services) schedulerstate.TaskState {
	t.Helper()
	got, err := svc.SchedulerState().Load(context.Background(), []string{"tick"})
	require.NoError(t, err)
	return got["tick"]
}

// lastRunAfter reports whether the last successful run of "tick" started
// strictly after want.
func lastRunAfter(svc *appbuild.Services, want time.Time) bool {
	got, err := svc.SchedulerState().Load(context.Background(), []string{"tick"})
	if err != nil {
		return false
	}
	ts, ok := got["tick"]
	return ok && ts.Active == nil && ts.LastRun.After(want)
}

// schedulerSettled reports whether a run of "tick" has ended, successfully or
// not.
func schedulerSettled(svc *appbuild.Services) bool {
	got, err := svc.SchedulerState().Load(context.Background(), []string{"tick"})
	if err != nil {
		return false
	}
	ts, ok := got["tick"]
	return ok && ts.Active == nil && (!ts.LastRun.IsZero() || ts.Failures > 0)
}

// discardTestLogger keeps expected failure output out of the test log.
func discardTestLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// listNotes returns the runs property of every note, so a test can assert on
// what the script actually wrote.
func listNotes(t *testing.T, svc *appbuild.Services) []string {
	t.Helper()

	ctx := context.Background()
	var out []string
	for e, err := range svc.Store().ListEntities(ctx, store.EntityQuery{Type: "note"}) {
		require.NoError(t, err)
		runs, _ := e.Properties["runs"].(string)
		out = append(out, runs)
	}
	return out
}

// settleFor is a small grace period for asynchronous completion. The scheduler
// waits on its own completion channel, so this is only a guard against a
// pathological scheduling delay rather than a real synchronization point.
const settleFor = 5 * time.Second
