---
id: BUGA-3HHW7C
type: bug-analysis-checklist
title: 'Analysis: Failed scheduled task re-runs every tick — a daily job executes every 60s until it succeeds'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced with a throwaway test against the **real** `doExecuteTask` (not the
success-simulating `executeTaskFunc` override): a `day` task pointing at a
missing script, driven through 10 tick calls to `runDueTasks` one minute apart,
all within the same calendar day.

Result: **`daily task executed 10 times within one day`** — one execution per
tick, want 1. Confirms path 1 (no state entry, `!recorded` "first run" branch).

Path 2 (stale entry after an earlier success) follows from the same omission:
`IsDue(lastRun, now)` keeps seeing the old `lastRun`. Both share one cause.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

**why1** — `doExecuteTask` returns on script error before writing
`s.state.Tasks[task.Name]` (`internal/scheduler/scheduler.go:228-237`), so the
failed attempt leaves no trace.

**why2** — `runDueTasks` gates re-execution *solely* on that map: absent key →
"first run, executing immediately"; present key → `IsDue(lastRun, now)`. With no
write on failure, neither branch can advance, so the task stays permanently due.

**why3** — the state field means "last **successful** run" but is used as if it
meant "last **attempt**". Those coincide only while nothing fails, so the model
is correct precisely until the first error — the case it needed to handle.

**why4** — there was no failure policy to encode. `docs/scheduled-tasks.md:308`
documents only that failures are logged at ERROR and the scheduler keeps
running; retry cadence was never specified, so "do nothing on failure" was
implemented literally and the tick rate became the de-facto retry interval.

**why5 (systemic)** — a single scalar (`map[string]time.Time`) was asked to
encode a two-dimensional state: *when did this last run* and *is it currently
failing*. Success-only persistence made the failure dimension unrepresentable,
so it silently collapsed onto the tick loop. The fix restores the missing
dimension explicitly (attempt time + consecutive-failure count) rather than
overloading the timestamp further.

**Test-coverage contributor**: `TestDoExecuteTask_PullsLuaWriteDeps`
(`scheduler_test.go:428-449`) already drives a *failing* execution, but asserts
only the deps call count and never inspects the state side effect. The one test
touching the failure path was blind to it. Separately, `newTestScheduler`'s
`executeTaskFunc` override (`:133-137`) always simulates **success**, so no
existing test could observe failure scheduling at all.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

### Approach (settled with user)

A single fixed backoff ladder that **replaces the schedule while a task is
failing**. One rule for every task — no thresholds, no per-schedule branching,
no schedule-derived formula.

**Ladder**: `5m, 10m, 20m, 40m, 80m`, then capped at **2h**, repeating.

**While a task is failing, the ladder is the ONLY trigger.** The normal
`IsDue`/first-run branches are skipped entirely, so a failing task fires exactly
once per ladder step — never on its schedule, and never before the step is due:

```text
if failing (nextRetry set):
    run only if now >= nextRetry     // schedule ignored completely
else:
    existing first-run / IsDue logic
```

Consequences of this being one uniform rule (both intended):

- `every: 5m` failing at 09:00 → attempts 09:05, 09:15, 09:35, 10:15, 11:35,
then every 2h. It **slows down** — no 5m hammering while broken.
- `every: day` failing at 09:00 → the same steps. It **speeds up** relative to
daily, so an intermittent failure recovers without waiting 24h.

**Reset on success ONLY.** Not on elapsed scheduled slots: for a 5m task a slot
passes every 5 minutes, so a slot-based reset would clear the counter constantly
and the ladder could never climb past its first step — reintroducing exactly the
hammering this removes.

**Escalating log severity** off the consecutive-failure count (user request):
WARN for early failures, ERROR once retries are clearly not helping, so an
intermittent blip reads differently from a persistently broken job.

**Secondary fix** (agreed): record the run's **start** time, not completion
time, closing the 23:59-crossing-midnight skip and interval drift.

### Accepted behaviour change

A daily task that fails at 09:00 and recovers on the 11:35 retry has then run
for that day, so it will not run again until tomorrow — the retry **is** that
day's run, not an extra one. A recovered run can therefore land hours after its
nominal slot. Intended.

### Rejected during design (recorded so it is not reintroduced)

- **A "short task" interval threshold** (e.g. skip retries below 1h). Redundant
and it special-cased daily vs 5m tasks; the user asked for one uniform ladder.
Dropping it also removes the need for a `nominalInterval()` helper, since
nothing has to derive a duration for `dayKind`/`weekdayKind` (which carry none —
`interval` is zero for them, `config.go:48-53`).
- **"Retry only if the step precedes the next scheduled slot."** This suppresses
the ladder entirely for short-interval tasks — every step >= 5m lands at or
after a 5m task's next slot — so a failing 5m task would keep firing every 5m
instead of backing off. Directly contrary to the requirement.
- **Reset the ladder at the next scheduled slot.** Destructive for
short-interval tasks, as above.

### State-file compatibility

`State` is `map[string]time.Time` under key `tasks` (`state.go:12-14`).
`json.Unmarshal` ignores unknown fields (no `DisallowUnknownFields`), and absent
fields take zero values — so adding sibling maps for failure count and
next-retry time is backward compatible. Two cautions from research: `parseState`
nil-guards only `s.Tasks` (`state.go:26-28`), so any new map needs the same
guard; and an older binary writing the file back **drops** the new fields (no
round-trip preservation) — acceptable, since the effect is only a reset ladder.

### Files to modify

| File | Change |
| --- | --- |
| `internal/scheduler/state.go` | Add failure-count + next-retry maps; nil-guard them in `parseState` |
| `internal/scheduler/scheduler.go` | Record attempt on failure; start-time stamp; retry gate in `runDueTasks`; ladder; escalating log severity |
| `internal/scheduler/scheduler_test.go` | Regression tests (below); failure-injecting override |
| `internal/scheduler/state_test.go` | Old-file forward-compat + round-trip for new fields |
| `docs-project/entities/guides/GUIDE-scheduled-tasks.md` | Retry/backoff docs — **source of truth**; `docs/scheduled-tasks.md` is auto-generated |

### Regression test plan (AM-scheduler-failed-task-not-rescheduled-immediately)

Existing helpers reuse: `newTestScheduler`, `dailySchedule()`,
`intervalSchedule(d)`, `mockWorkspace`, `discardLogger`. A new failure-injecting
`executeTaskFunc` is required — the current one only simulates success.

1. **No prior state, day schedule** — 10 ticks 1m apart within one day: exactly
1 execution (the exact scenario reproduced above as 10).
2. **Stale state after success** — succeeded yesterday, fails today: runs once
today, then follows the ladder rather than every tick.
3. **Ladder steps** — failing daily task: attempts at +5m, +10m, +20m, +40m,
+80m, then every 2h; assert no attempt strictly between ladder steps.
4. **Short-interval task backs OFF** — a failing `5m` task follows the same
ladder and does **not** keep firing every 5m.
5. **Schedule suppressed while failing** — a failing task does not fire on its
normal schedule, only on ladder steps (no double-firing).
6. **Reset on success only** — the ladder clears after a successful run;
elapsed scheduled slots while still failing do **not** reset it.
7. **Start-time stamp** — task starting 23:59 running past midnight does not
skip the next day.
8. **State compat** — a state file with only `tasks` loads with zero-valued new
fields and no panic.

### Related areas checked

- `internal/cli/sync/state.go:26` has a separate `sync-state.json` with its own
reader/writer; it does **not** share this success-only-persistence pattern.
- No other package reads `scheduler-state.json` — `stateFile` is unexported and
the only readers/writers are `scheduler.go:241,255`. Blast radius is confined to
`internal/scheduler` plus the generated doc.
- `docs/scheduled-tasks.md:228-230` separately claims *"Week tasks: missed if
the ISO week changed since the last run"*, which does **not** match
`weekdayKind`'s actual `mostRecentWeekday` logic (`config.go:73-74`).
Pre-existing doc inaccuracy, unrelated to this bug — noted, not fixed here.
