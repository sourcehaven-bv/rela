---
id: IMPL-4PHH6L
type: implementation-checklist
title: 'Implementation: Failed scheduled task re-runs every tick — a daily job executes every 60s until it succeeds'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

### Changes

| File | Change |
| --- | --- |
| `internal/scheduler/state.go` | `Failures map[string]int` + `NextRetry map[string]time.Time` (both `omitempty`); each nil-guarded independently in `parseState` |
| `internal/scheduler/scheduler.go` | Retry gate at the top of `runDueTasks`; `recordFailure`; `retryDelay` ladder; start-time stamp; success clears the ladder; ladder constants; package doc |
| `internal/scheduler/scheduler_test.go` | 6 regression tests + `newRetryTestScheduler` / `tickFor` / `offsets` helpers |
| `internal/scheduler/state_test.go` | Old-file compat + retry-field round-trip |
| `docs-project/entities/guides/GUIDE-scheduled-tasks.md` | New "Failure Handling and Retries" section (source of truth) |
| `docs/scheduled-tasks.md` | Regenerated via `scripts/generate-docs.sh` — **not** hand-edited (the RR-U2LC1P drift class) |

The gate is the whole fix: while `NextRetry` holds an entry it is the sole
trigger and the `IsDue`/first-run branches are skipped, so a failing task fires
once per ladder step and never on its schedule.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Assertions are expressed against `baseRetryDelay` / `maxRetryDelay` rather than
literal `5m` / `2h`, so retuning the ladder does not require rewriting the
tests. Ladder offsets are computed from the test's own `base` time.

A new `newRetryTestScheduler` was required: the existing `newTestScheduler` has
a **fixed** clock and an `executeTaskFunc` that only ever simulates **success**,
so failure scheduling was unobservable through it. The new helper takes a
mutable clock and a `fail()` predicate, and delegates to the real
`recordFailure` bookkeeping rather than reimplementing it.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Verified against the **real `rela scheduler` binary** in a scratch project
(`/tmp/sched-e2e`, since removed) with `every: day` and a Lua script calling
`error()`.

**1. Hot loop fixed.** Ran for **80s — one full tick past the 60s boundary**.
Task executed **exactly once**; pre-fix it fires at 0s and 60s.

```text
level=INFO msg="first run, executing immediately" name=broken-daily
level=WARN msg="task failed" name=broken-daily failures=1 retry_in=5m0s \
  retry_at=2026-08-13T23:12:04+02:00 error="scripts/broken.lua:1: simulated..."
level=INFO msg="scheduler started" tasks=1
level=INFO msg="scheduler stopped"          <- 80s later, no second run
```

State after the failure — note `tasks` is **empty**, i.e. the failure correctly
did not stamp a last-run time:

```json
{"tasks": {}, "failures": {"broken-daily": 1},
 "next_retry": {"broken-daily": "2026-08-13T23:12:04.790786+02:00"}}
```

**2. Backoff survives restart** (the crucial regression). Restarted the
scheduler with the 5m retry still pending: **0 executions** in 70s. Pre-fix an
empty `tasks` map meant "first run" and it would fire immediately on every
restart.

**3. Success resets the ladder.** Backdated `next_retry` into the past and fixed
the script:

```text
level=INFO msg="retrying failed task" name=broken-daily failures=1 scheduled_for=...
recovered ok
level=INFO msg="task completed" name=broken-daily duration=1.2875ms
```

State afterwards — `failures`/`next_retry` gone entirely (`omitempty`), `tasks`
carrying the run's start time, task back on its normal daily schedule:

```json
{"tasks": {"broken-daily": "2026-08-13T23:09:47.615122+02:00"}}
```

**4. WARN vs ERROR escalation** confirmed live: failure #1 logged at `WARN` with
`retry_in=5m0s`, escalating to `ERROR` at `persistentFailureThreshold` (4).

**5. Original reproduction re-run.** The scratch test that produced *"daily task
executed 10 times within one day"* now yields **2** — the initial run plus the
5m ladder step, both inside its 10-minute window. It asserted `want 1` (the
pre-retry design), so its failure is expected; the committed
`TestRunDueTasks_failingTaskDoesNotHotLoop` asserts 2 and checks the gap is
`baseRetryDelay`. Scratch file deleted.

### Automated gates

| Gate | Result |
| --- | --- |
| `go test ./...` | PASS (full repo) |
| `go test -race ./internal/scheduler/` | PASS |
| `just lint` | 0 issues |
| `just coverage-check` | PASS — scheduler 80.4% |
| `just arch-lint` | OK, no warnings |
| `just plimsoll` | PASS |

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Ladder timings are named constants (`baseRetryDelay`, `maxRetryDelay`,
`persistentFailureThreshold`) referenced by both the implementation and the
tests, so there are no magic durations in either. `retryDelay` is a pure
function, unit-tested in isolation across the full ladder including the
defensive `failures < 1` case and a `failures=50` overflow check.

The failure path now logs *more* than before (count, next delay, absolute retry
time) — the error is still surfaced in full, at a severity that reflects how
long the task has been failing.

**Scope note:** `docs/scheduled-tasks.md:228-230` claims week tasks trigger on
ISO-week change, which does not match `mostRecentWeekday` (`config.go:73-74`).
Pre-existing and unrelated to this bug; left untouched and recorded in the
analysis rather than silently swept in.

**Unrelated blocker fixed to proceed:**
`tickets/entities/automated-measures/AM-date-property-write-roundtrip.md` had an
unquoted `due: 2026-08-12` inside its `description:` scalar, which made the
frontmatter invalid YAML and broke *all* entity creation in the tickets project
("collect existing IDs: failed to parse frontmatter"). Quoted the value.
