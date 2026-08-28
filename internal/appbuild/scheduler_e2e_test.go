package appbuild_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/cli"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
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

	// The scheduler persists its ladder to .rela/scheduler-state.json, which is
	// the same surface an operator would inspect.
	state := readSchedulerState(t, svc)
	require.Contains(t, state, `"failures"`,
		"a failing script must advance the retry ladder")
	require.Contains(t, state, `"next_retry"`,
		"a failed task must have a pending retry (BUG-ZKK2UL)")
	require.NotContains(t, state, `"tasks": {\n    "tick"`,
		"a failed run must not stamp a last-successful-run time")
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
	// without waiting for a tick. Stop as soon as the state file shows it.
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
// Between them it rewinds the persisted last-run stamp so the task is due
// again, rather than waiting out a real interval.
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

	// Rewind so the task is due, then run the SAME scheduler again.
	rewindLastRun(t, svc)

	runUntil(t, s, func() bool {
		notes := listNotes(t, svc)
		return len(notes) > 0 && notes[0] == "2"
	}, "second run did not complete")
}

// schedulerStateFile is the scheduler's state file inside .rela/. The
// scheduler names it in its own unexported const; duplicated here because
// this is an external test package.
const schedulerStateFile = "scheduler-state.json"

// rewindLastRun pushes the task's last-run stamp far enough into the past that
// the scheduler considers it due on its next evaluation.
func rewindLastRun(t *testing.T, svc *appbuild.Services) {
	t.Helper()

	ctx := context.Background()
	data, err := svc.State().Get(ctx, schedulerStateFile)
	require.NoError(t, err)

	old := time.Now().Add(-48 * time.Hour).Format(time.RFC3339Nano)
	var st map[string]map[string]any
	require.NoError(t, json.Unmarshal(data, &st))
	require.Contains(t, st, "tasks")
	st["tasks"]["tick"] = old

	out, err := json.Marshal(st)
	require.NoError(t, err)
	require.NoError(t, svc.State().Put(ctx, schedulerStateFile, out))
}

// schedulerSettled reports whether the scheduler has written an outcome —
// either a successful run or a failure — to its state file.
func schedulerSettled(svc *appbuild.Services) bool {
	data, err := svc.State().Get(context.Background(), schedulerStateFile)
	if err != nil {
		return false
	}
	body := string(data)
	return strings.Contains(body, `"tick"`)
}

// readSchedulerState returns the persisted scheduler state as text.
func readSchedulerState(t *testing.T, svc *appbuild.Services) string {
	t.Helper()

	data, err := svc.State().Get(context.Background(), schedulerStateFile)
	require.NoError(t, err)
	return string(data)
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
