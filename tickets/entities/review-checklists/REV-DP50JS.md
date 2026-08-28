---
id: REV-DP50JS
type: review-checklist
title: 'Review: Failed scheduled task re-runs every tick — a daily job executes every 60s until it succeeds'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./...` passes (full repo)
- [x] `go test -race ./internal/scheduler/` passes
- [x] `just lint` clean (0 issues)
- [x] `just coverage-check` passes — scheduler **84.8%** (was 80.4% pre-review)
- [x] `just arch-lint` — no warnings
- [x] `just plimsoll` — passes

## Code Review

- [x] cranky-code-reviewer invoked
- [x] All findings logged as review-response entities
- [x] All critical and significant findings addressed

**9 findings, all addressed** (3 critical, 3 significant, 2 minor, 1 nit):

| ID | Severity | Finding |
| --- | --- | --- |
| RR-F6182G | critical | Start-time test never entered `doExecuteTask` |
| RR-QOSJZ5 | critical | No test pinned "a failure must not touch `state.Tasks`" |
| RR-R6YXKM | critical | Far-future `next_retry` wedged a task permanently and silently |
| RR-3BCWQ4 | significant | Test double reimplemented the success path |
| RR-BVQVTT | significant | `New` left `state` nil |
| RR-KIGI09 | significant | `State` godoc claimed false forward compatibility |
| RR-7GYJ60 | minor | Orphaned state never pruned |
| RR-14NHU6 | minor | `tickFor` attributed post-execution clock times |
| RR-UF59GR | nit | Dead `min()`, missing `t.Run`, `errBoom` placement, doc sample |

### The review's central catch

The reviewer ran **mutation testing** and found the test suite could not detect
reintroduction of *either* bug this ticket fixes. I reproduced both
independently before acting:

- Reverting the start-time fix to `s.now()` → suite **green**.
- Injecting `state.Tasks[name] = start` into `recordFailure` → suite **green**.

One root cause (RR-3BCWQ4): the test double **hand-copied** the success
bookkeeping instead of calling it, so assertions validated the copy. Fixed by
extracting `recordSuccess` and having both the production path and the double
call it, plus two new tests driving the **real** `doExecuteTask` against a real
`script.Engine` and a real `.lua` file on disk.

**Post-fix mutation results** — all now caught:

| Mutation | Before | After |
| --- | --- | --- |
| Stamp completion instead of start | survived | **FAILS** `TestDoExecuteTask_recordsStartTimeNotCompletion` |
| Failure stamps `state.Tasks` | survived | **FAILS** `TestDoExecuteTask_failureDoesNotCountAsRun` |
| Remove the retry gate (the original bug) | — | **FAILS 4 tests** |

### Latent bug found while fixing RR-BVQVTT

Initialising `state` in `New` exposed a panic: `pruneOrphanedState` dereferenced
a nil `config` in an existing test. It now returns early — without that guard,
an empty config would have treated **every** entry as orphaned and wiped the
state file.

## Acceptance Verification

- [x] Each acceptance criterion verified with evidence

| Criterion | Result | Evidence |
| --- | --- | --- |
| Failing task no longer runs every tick | **PASS** | Real binary, 80s run (past the 60s boundary): exactly 1 execution |
| Failure does not count as a run | **PASS** | `"tasks": {}` after failure; pinned by test |
| Ladder 5m→10m→20m→40m→80m→2h | **PASS** | `TestRunDueTasks_retryLadder` asserts exact offsets |
| Ladder replaces schedule while failing | **PASS** | `TestRunDueTasks_scheduleSuppressedWhileFailing`; gate removal fails 4 tests |
| Failing 5m task backs off | **PASS** | `TestRunDueTasks_shortIntervalTaskBacksOff` |
| Reset on success only | **PASS** | Live: `failures`/`next_retry` cleared, task back on daily schedule |
| Backoff survives restart | **PASS** | Live: 0 executions on restart with 5m retry pending |
| WARN → ERROR escalation | **PASS** | Live: `level=WARN ... failures=1`; ERROR at threshold 4 |
| Start time, not completion | **PASS** | Real-path test; mutation now caught |
| Old state files still load | **PASS** | `TestParseState_oldFileWithoutRetryFields` |
| Wedged retry self-heals | **PASS** | Live: year-2126 `next_retry` → WARN + immediate retry |
| Orphaned state pruned | **PASS** | Live: `pruned state for tasks no longer configured tasks=[removed-task]` |

Final end-to-end run seeded a state file with **both** a wedged far-future retry
and an orphaned task, and confirmed: prune logged, clamp warned and retried, and
the ladder resumed at the correct rung (`failures=2` → 3, `retry_in=20m`) across
a restart.

## Notes

`docs/scheduled-tasks.md` was regenerated via `scripts/generate-docs.sh` from
`docs-project/entities/guides/GUIDE-scheduled-tasks.md` — never hand-edited (the
RR-U2LC1P drift class).

Deliberately **not** fixed here: `docs/scheduled-tasks.md` claims week tasks
trigger on ISO-week change, which does not match `mostRecentWeekday`.
Pre-existing and unrelated; recorded in the analysis rather than silently swept
in.
