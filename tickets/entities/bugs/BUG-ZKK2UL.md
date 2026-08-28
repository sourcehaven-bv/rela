---
id: BUG-ZKK2UL
type: bug
title: Failed scheduled task re-runs every tick — a daily job executes every 60s until it succeeds
description: doExecuteTask returns early on script error without recording a run timestamp, so a failing task stays perpetually due and is retried on every 60s tick instead of at its configured interval.
priority: high
effort: s
why1: doExecuteTask returns on script error before writing s.state.Tasks[task.Name] (scheduler.go:228-237), so a failed attempt leaves no trace in state.
why2: runDueTasks gates re-execution solely on that map — absent key means 'first run, execute immediately', present key means IsDue(lastRun, now). With no write on failure neither branch can advance, so the task stays permanently due and retries at the 60s tick rate.
why3: The state field means 'last successful run' but is used as if it meant 'last attempt'. Those coincide only while nothing fails, so the model is correct precisely until the first error — the case it needed to handle.
why4: There was no failure policy to encode. docs/scheduled-tasks.md documents only that failures are logged at ERROR and the scheduler keeps running; retry cadence was never specified, so 'do nothing on failure' was implemented literally and the tick rate became the de-facto retry interval.
why5: 'A single scalar (map[string]time.Time) was asked to encode two dimensions: when a task last ran, and whether it is currently failing. Success-only persistence made the failure dimension unrepresentable, so it silently collapsed onto the tick loop.'
prevention: 'Restore the missing dimension explicitly (attempt timestamp + consecutive-failure count) rather than overloading the timestamp further, and pin it with AM-scheduler-failed-task-not-rescheduled-immediately. Root test-coverage gap: the one existing test touching the failure path (TestDoExecuteTask_PullsLuaWriteDeps) asserted only a call count and never the state side effect, and the shared executeTaskFunc test override simulated success exclusively — so no test could observe failure scheduling. Any state that gates re-execution needs a test asserting the failure path''s persistence, not just the success path''s.'
status: done
---

## Symptom

A scheduled task whose Lua script fails is re-executed on **every scheduler tick
(60s)** rather than at its configured schedule. A task configured `every: day`
runs ~1440 times per day while it keeps failing.

Two distinct paths produce this, both in `internal/scheduler/scheduler.go`:

**1. First-ever run fails (no state entry).** `runDueTasks` branches on
`lastRun, recorded := s.state.Tasks[task.Name]`. On failure `doExecuteTask`
returns before writing `s.state.Tasks[...]`, so `recorded` stays false. The next
tick logs "first run, executing immediately" again. This loops forever — the
schedule is never consulted.

**2. Run fails after an earlier success (stale state entry).** `lastRun` keeps
its old value, so `Schedule.IsDue(lastRun, now)` stays true on every subsequent
tick until a run finally succeeds. Retry cadence is the tick interval, not the
schedule.

The effect is worst for a script that fails *after* doing partial work, or one
that costs real money/time (an AI call, an external API): a broken daily job
becomes a 60-second hot loop, and any side effects it performs before failing
are repeated 1440x/day.

## Root cause

`doExecuteTask`:

```go
if err != nil {
    s.logger.Error("task failed", ...)
    return          // <-- no state write
}
...
s.state.Tasks[task.Name] = s.now()
s.saveState(ctx)
```

State is the *only* thing gating re-execution, and it is written solely on
success. "Last successful run" is being used as if it meant "last attempt".
Those coincide only while nothing fails.

## Secondary defect (same function)

The success path records `s.now()` **after** the script finishes — the
completion time, not the start time. For a `day` schedule the due check is
`truncateToDay(now) != truncateToDay(lastRun)`, so a task starting at 23:59 that
runs for two minutes stamps the *next* day and silently skips that day's
execution. Recording the start time also avoids drift on interval schedules.

## Not in question

`docs/scheduled-tasks.md` documents only that failures are logged at ERROR and
that the scheduler keeps running. No retry policy is specified anywhere, so the
current hot-loop is unintended behaviour rather than a deliberate retry design.
Deciding the intended policy (retry at the next scheduled slot vs. bounded
backoff) is part of this ticket.

## Reproduction

1. `schedules.yaml` with a task `every: day` pointing at a script that
errors (a missing file is enough — `ExecuteFile` returns an error).
2. Run `rela scheduler` with no `.rela/scheduler-state.json`.
3. Observe "first run, executing immediately" logged every 60 seconds.

`TestDoExecuteTask_PullsLuaWriteDeps` already drives a failing execution (script
`missing.lua`) but asserts only the deps call count — it never checks the state
side effect, which is why this went unnoticed.
