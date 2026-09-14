---
id: BUG-PRFSTS
type: bug
title: Scheduler e2e tests race TempDir cleanup; saveState outlives the assertion
description: >-
  TestSchedulerCmd_EndToEnd fails intermittently in CI with "TempDir RemoveAll
  cleanup: unlinkat /tmp/TestSchedulerCmd_EndToEnd.../001/.rela: directory not
  empty". The test synchronises on the note the task creates, but recordSuccess
  calls saveState AFTER that note lands, writing .rela/scheduler-state.json.
  So require.Eventually returns, the test cancels and finishes, and t.TempDir's
  RemoveAll races the state write into a directory it has already scanned.
  Observed on the ubuntu SQLite Backend job; does not reproduce on macOS.
priority: low
status: done
why1: TestSchedulerCmd_EndToEnd failed in CI on a PR that touches neither appbuild nor the scheduler.
why2: t.TempDir's RemoveAll found .rela non-empty, so a file appeared in it after cleanup began.
why3: The scheduler wrote .rela/scheduler-state.json after the test had stopped waiting.
why4: The test waits on the note the task creates, but that is not the task's last side effect -- recordSuccess persists state after the note is written, so the observable the test picked is not the one that means "done".
why5: Waiting on a visible side effect is the obvious way to synchronise, and it is right until the code under test has a LATER side effect the test does not know about. Nothing named the state write as part of "the run finished", so no test could wait for it.
prevention: >-
  When a test waits for background work to finish, wait on that work's LAST
  side effect, not its most convenient one. Where the last effect is not
  observable, the production type needs to expose a done signal rather than
  have tests guess. Anything writing under a t.TempDir must be joined before
  the test returns; the cost of getting it wrong is a flake that appears on
  one OS and reproduces on no developer's machine.
---

## Symptom

```
--- FAIL: TestSchedulerCmd_EndToEnd (0.00s)
    testing.go:1464: TempDir RemoveAll cleanup: unlinkat
      /tmp/TestSchedulerCmd_EndToEnd1545794715/001/.rela: directory not empty
FAIL	github.com/Sourcehaven-BV/rela/internal/appbuild	0.343s
```

Seen on the `SQLite Backend` job (ubuntu). 15 local runs on macOS did not
reproduce it, which is consistent with a timing window rather than an
OS-specific bug.

## Root cause

`internal/appbuild/scheduler_e2e_test.go` waits for the task's note:

```go
require.Eventually(t, func() bool {
    notes := listNotes(t, svc)
    return len(notes) == 1 && notes[0] == "1"
}, settleFor, 20*time.Millisecond, ...)
cancel()
```

But `Scheduler.recordSuccess` (`internal/scheduler/scheduler.go:459`) persists
state *after* the script has produced that note:

```go
s.state.Tasks[task.Name] = start
delete(s.state.Failures, task.Name)
delete(s.state.NextRetry, task.Name)
s.saveState(ctx)          // writes .rela/scheduler-state.json
```

The note is the test's synchronisation point; the state file is the task's
actual last write. Between them the test may cancel, return, and start
`TempDir` cleanup — which then trips over the state file appearing underneath
it.

`cancel()` does not close the window: `Run` returns on `ctx.Done()`, but the
in-flight `runDueTasks` call that is mid-`recordSuccess` is not joined by it.

## Fix applied

Option 1: the tests now wait for the state file as well as the note, since it
is the run's real completion marker and `schedulerSettled` already reads it for
the other cases in this file.

`TestSchedulerCmd_EndToEnd` gains a second `require.Eventually` on
`schedulerSettled`. The second run in `TestScheduler_EndToEnd_RepeatedRunsAccumulate (via runSchedulerTwice)`
needed more care: `rewindLastRun` leaves `"tick"` in the file, so
`schedulerSettled` is ALREADY TRUE before that run starts and would have been a
no-op barrier. `lastRunAfter` waits for the stamp to move strictly forward off
the rewound value instead.

Load-bearingness was measured, not assumed: instrumented, the new barrier
blocked in 25 of 25 runs and was a no-op in 0.

## Fix NOT applied, and why

Option 2 — having `Run` join in-flight task execution before returning, so
`cancel()` plus `<-done` is a genuine barrier for every caller rather than just
these tests — is the stronger fix and remains open. It changes production
shutdown semantics and deserves its own review, so it is deliberately out of
scope here.

That means the underlying asymmetry survives: `Run` still returns while a
`runDueTasks` call may be mid-`recordSuccess`. Only the tests are now defended
against it.
