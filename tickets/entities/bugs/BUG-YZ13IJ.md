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
status: backlog
---

## Symptom

A scheduled task declared `every: day` executed **1,440 times per day** — once
per scheduler tick (60 s). Each run committed an entity then aborted on a denied
`create_relation`, leaking one orphan entity per tick. Reported from production
at 11,222 orphans over ~7 days.

## Reproduction

Confirmed locally (pgstore). With `RELA_SCHEDULER_TICK=50ms`:

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

`RELA_SCHEDULER_TICK` (env-only debug seam, default unchanged at 60 s) was added
during investigation to compress days of ticking into seconds. Keep or drop with
the fix, but a test-visible tick override is what makes criterion 3 practical.

## Related

- Analyze OOM ticket (`_analyze` unbounded retention) — the amplifier that turned
this leak into a host-level outage.
- `entitymanager.collectAllIDs` full-content scan per create — each leaked entity
cost a full-table scan, so the leak also degraded write latency as it grew.
