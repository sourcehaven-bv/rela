---
id: IMPL-PRFSTS
type: implementation-checklist
title: 'Implementation: Scheduler e2e tests race TempDir cleanup; saveState outlives the assertion'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: the change IS test code — the fix is a corrected synchronisation barrier, not new production behaviour)
- [x] ~~Integration tests written~~ (N/A: same reason; the affected tests are already the e2e layer)
- [x] Happy path implemented — `TestSchedulerCmd_EndToEnd` now waits for `schedulerSettled` (the `.rela/scheduler-state.json` write) after waiting for the note, so the test returns only once the run's LAST write has landed
- [x] Edge cases handled — the second-run case in `TestScheduler_EndToEnd_RepeatedRunsAccumulate (via runSchedulerTwice)` could not use plain `schedulerSettled`: `rewindLastRun` leaves `"tick"` in the file, so that predicate is already true before the second run starts. Added `lastRunAfter`, which waits for the stamp to move strictly forward off the rewound value
- [x] Error handling in place — `lastRunAfter` returns false (keeps waiting) on a missing or unparseable state file rather than failing the poll, matching `schedulerSettled`'s existing behaviour

## Test Quality

- [x] Using fixture builders — reuses the file's existing `schedulerSettled`, `rewindLastRun` and `runUntil` helpers rather than introducing a parallel mechanism
- [x] No hardcoded values in assertions — `lastRunAfter` compares against the stamp `rewindLastRun` actually wrote, returned from that helper, instead of a literal
- [x] Only specifying values that matter — the barrier asserts ordering (state written after note), not the state file's contents
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolated assertion strings added)
- [x] Property comparisons use original object — `lastRunAfter(svc, rewound)` takes the rewound `time.Time` returned by `rewindLastRun`, so the two cannot drift

## Manual Verification

- [x] Feature manually tested end-to-end — 40 consecutive runs of `go test -tags sqlite -run TestScheduler -count=40 ./internal/appbuild/` pass; full `internal/appbuild` and `internal/scheduler` suites pass under `-shuffle=on` on both the default and `sqlite` tags
- [x] Each acceptance criterion verified — instrumented the new barrier and measured it: it BLOCKED in 25 of 25 runs and was a no-op in 0. The state write genuinely lands after the note every time, which is exactly the window that was racing cleanup
- [x] Edge cases verified — audited every wait in `scheduler_e2e_test.go`; the two note-based waits were the only ones stopping before the state write. The other cases already used `schedulerSettled`

**Verification Evidence:** Observed three times on 2026-09-08 across unrelated
PRs (#1488 twice, #1553 once), always on the ubuntu `SQLite Backend` job:
`TempDir RemoveAll cleanup: unlinkat /tmp/TestSchedulerCmd_EndToEnd.../001/.rela:
directory not empty`. Root cause read from the source rather than guessed:
`Scheduler.recordSuccess` (`internal/scheduler/scheduler.go:459`) calls
`saveState` AFTER the script has produced the note the test waits on, and
`cancel()` does not join an in-flight `runDueTasks`.

The race does not reproduce on macOS — 60 runs with the barrier removed still
passed — so the barrier's load-bearingness was established by instrumentation
(25/25 blocked) rather than by reproducing the failure locally. That asymmetry
is stated here rather than hidden: the fix is justified by the ordering it
enforces, which is observable, not by a local red-to-green transition, which is
not.

## Quality

- [x] Code follows project patterns — uses the file's existing settle-predicate idiom (`require.Eventually` over a `func() bool`), its `schedulerStateFile` const, and `schedulerSettled`'s signature shape: `lastRunAfter` takes only the service, not `*testing.T`, because a predicate on a 20ms poll loop should not hold the test handle
- [x] Checked for DRY opportunities — the stronger fix is having `Scheduler.Run` join in-flight task execution so `cancel()` plus `<-done` is a real barrier for every caller, not just these tests. Deliberately NOT done here: it changes production shutdown semantics and deserves its own review. Recorded in the bug's fix-direction section and in the measure
- [x] No security issues introduced — test-only change; no production code path altered
