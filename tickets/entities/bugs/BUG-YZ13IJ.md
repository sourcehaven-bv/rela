---
id: BUG-YZ13IJ
type: bug
title: 'Failed scheduled task re-runs every tick: !recorded branch bypasses IsDue, so any schedule kind fires at tick rate'
description: A scheduled task that fails before ever succeeding never stamps last-run, so runDueTasks takes the !recorded branch on every tick and executes without consulting IsDue. A task with a partially-denied ACL that commits an entity then aborts leaks one entity per tick, unbounded.
priority: critical
effort: s
why1: A scheduled task created 1,440 orphan entities per day instead of 1.
why2: doExecuteTask returns early on error, so state.Tasks[name] is never stamped.
why3: runDueTasks treats an unstamped task as !recorded and executes it immediately, bypassing IsDue entirely — so the declared schedule is irrelevant.
why4: State conflates 'last run' with 'last SUCCESSFUL run'; the model assumed a failed run has no side effects.
why5: That assumption was never re-derived when Lua scripts gained write bindings. A failing task is now a writer, so retry cadence became a data-integrity concern with no test covering the error path across ticks.
status: wont-fix
reason: "Duplicate of BUG-ZKK2UL, which was independently filed and fixed in #1319 (2af4f266) while this investigation was in flight. Verified fixed on develop, including this ticket's broader finding that the amplification was never daily-specific."
resolution: "No code change needed. #1319 added a retry ladder (NextRetry/Failures) whose gate precedes the schedule check in runDueTasks, making the !recorded fall-through unreachable for a failing task. Verified across daily and interval schedules: 2 executions per 9 minutes of 1-minute ticks, not 9."
---

## Symptom

A scheduled task declared `every: day` executed **1,440 times per day** — once
per scheduler tick (60 s). Each run committed an entity then aborted on a denied
`create_relation`, leaking one orphan entity per tick. Reported from production
at 11,222 orphans over ~7 days.

## Reproduction

Confirmed locally (pgstore), with the scheduler's 60 s `tickInterval`
temporarily lowered to 50 ms so days of ticking compress into seconds
(see Notes — that override was NOT kept):

- **1,177 orphan `task` entities in 60 seconds**, 0 relations.
- Error identical on every tick:
`create relation error: forbidden: no role grants create on relations from type
"recurring" (rule_kind=role-grant rule_id=-)`

The reporter's 1,440/day matches exactly at the production 60 s tick.

Minimal setup: a metamodel with `recurring` + `task` and a `spawns` relation
`from: [recurring]`; an `acl.yaml` granting `create: [task]` but only `read` on
`recurring`; a script that creates a task then relates it back to the recurring
entity. `create_entity` commits, `create_relation` is denied, the task aborts.

## Root cause

`internal/scheduler/scheduler.go`. `doExecuteTask` returns before stamping
state:

```go
if err != nil {
    s.logger.Error("task failed", ...)
    return                          // state.Tasks[task.Name] never set
}
s.state.Tasks[task.Name] = s.now()
```

and `runDueTasks` short-circuits when there is no stamp:

```go
lastRun, recorded := s.state.Tasks[task.Name]
if !recorded {
    s.logger.Info("first run, executing immediately", "name", task.Name)
    s.executeTask(ctx, task)
    continue                        // IsDue is never consulted
}
if task.Every.IsDue(lastRun, now) { ... }
```

**The declared schedule is therefore irrelevant for a task that has never
succeeded.** The reproduction log shows `"first run, executing immediately"` on
*every* tick, never `"task due"`.

This is broader than the original report suggested. The reporter hypothesised
the `dayKind` branch of `IsDue` (`truncateToDay(now) != truncateToDay(lastRun)`)
as the cause, which would have limited the bug to `every: day`. In fact `IsDue`
is dead code on this path — `every: week`, `every: monday`, and `every: 2h` all
amplify to tick rate identically.

## Impact

Any persistently-failing scheduled task runs at tick rate (60 s) regardless of
its declared schedule. Where the script writes before failing, this is unbounded
data growth. The ACL misconfiguration that triggered the production incident is
the reporter's own, but a transient failure (network blip mid-script) produces
the same amplification for as long as it persists.

Downstream: the inflated entity table is what made `GET /api/v1/_analyze` fatal
(see Related) — 19 MB to 2,947 MB RSS. Neither defect alone took the host down.

## Fix direction

Track last-*attempt* separately from last-*success*, so a failed run still
advances the retry clock. Add exponential backoff on consecutive failures, and
consider a circuit-breaker that disables a task after N identical failures (the
analogue of Erlang's supervisor restart intensity). Fetching the schedule via
`IsDue` must happen on every path, including first-run-after-failure.

Note that wrapping each task in a transaction — the reporter's first suggestion
— is **not** the right fix: Lua scripts can call `rela.http` (30 s timeout) and
`rela.ai`, and CLAUDE.md forbids slow external I/O inside a `store.Store.Tx`
callback. Script authors need documented non-atomicity plus an idempotency
primitive, not a transaction that holds a pg advisory lock across arbitrary
network calls.

## Acceptance criteria

1. A task that fails does not re-execute on the next tick; the next execution
respects its declared schedule.
2. Holds for every schedule kind: `day`, weekday, `week`, and interval.
3. Consecutive failures back off rather than retrying at tick rate.
4. A task that has never succeeded is still eventually retried (no permanent wedge
from one early failure).
5. Successful runs are unaffected — existing scheduling behaviour is unchanged.

## Test plan

- **Regression test (must fail on current `develop`)**: a scheduled task whose
execution always errors; advance the clock by one tick; assert exactly one
execution, not two.
- Table-driven across all schedule kinds, asserting the amplification is absent for
each — this is what pins the finding that `IsDue` is bypassed rather than
mis-evaluated.
- Backoff timing test with an injected clock (`Scheduler.now` is already a seam).
- A task failing then succeeding resumes its normal cadence.

## Notes

**Reproducing this needs a tick override.** The bug is only observable across
many ticks, and `tickInterval` is a 60 s const, so a bare reproduction takes
hours. During investigation a `RELA_SCHEDULER_TICK` env seam was added to
compress that; it was **deliberately not kept**, because on its own it is a
debug knob with no caller, no test, and no bug it fixes.

Whoever takes this ticket should reintroduce it (or an equivalent) as part of
the fix, where it lands with a caller and a rationale. Acceptance criterion 3
(consecutive failures back off) is impractical to test without one.

Note the scheduler already has the two seams the regression test needs —
`Scheduler.now` for the clock and `executeTaskFunc` for execution — so only the
tick period is missing.

## Related

- Analyze OOM ticket (`_analyze` unbounded retention) — the amplifier that turned
this leak into a host-level outage.
- `entitymanager.collectAllIDs` full-content scan per create — each leaked entity
cost a full-table scan, so the leak also degraded write latency as it grew.

## Resolution — duplicate, already fixed

Closed `wont-fix` as a **duplicate of BUG-ZKK2UL**, which was filed and fixed
independently in **#1319** (`2af4f266`, "fix(scheduler): back off failed tasks
instead of retrying every tick") while this OOM investigation was still running.
Both tickets describe the same defect from different directions: ZKK2UL from
reading the scheduler, this one from the production leak it caused.

### Verified on develop

`recordFailure` now always writes `NextRetry`, and `runDueTasks` checks that
ladder **before** reaching the `!recorded` branch:

```go
if retryAt, retrying := s.state.NextRetry[task.Name]; retrying {
    if !now.Before(retryAt) { s.executeTask(ctx, task) }
    continue                     // never falls through to "first run"
}
```

The `!recorded` branch this ticket blamed still exists verbatim — but it is now
**unreachable for a failing task**, because a failure can no longer leave state
untouched. `recordFailure` also deliberately does not touch `s.state.Tasks`, so
the schedule still evaluates against the last *successful* run.

Backoff is a 5m → 2h doubling ladder, cleared only by success — with an explicit
comment that elapsed scheduled slots must not clear it, or a short-interval task
would never back off at all. That is a sharper reading than this ticket's
"add exponential backoff" and covers acceptance criteria 1-5.

### This ticket's one distinct claim, checked

This ticket argued the amplification was **not** `dayKind`-specific (the original
reporter's hypothesis) because `IsDue` was bypassed entirely. That is correct,
and the fix is correspondingly kind-independent: the `NextRetry` gate precedes
any schedule evaluation. Confirmed empirically with a throwaway probe across
`daily`, `interval 2h` and `interval 5m` — **2 executions per 9 minutes of
1-minute ticks in every case**, versus 9 pre-fix.

`TestRunDueTasks_failingTaskDoesNotHotLoop` pins this for `daily` only. A
table-driven variant across kinds would document kind-independence, but it
cannot fail independently — the gate returns before the schedule is consulted —
so it was not added. Noting the option rather than padding the suite.

### Left in place

The `RELA_SCHEDULER_TICK` note under Notes is now moot; #1319's tests use an
injected clock plus `executeTaskFunc`, which is the seam this ticket predicted
would be needed.
