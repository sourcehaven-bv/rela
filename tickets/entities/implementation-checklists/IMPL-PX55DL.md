---
id: IMPL-PX55DL
type: implementation-checklist
title: 'Implementation: Reload schedules and subscriptions without a restart'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (reload_test.go: AppliesEdits, UnchangedKeepsTheConfig, TaskDiff, BrokenSubscriptionsKeepSchedulesLive, MaybePrune_SkipsAnIncompleteConfig, StartBackground_RunsWithoutTasks)
- [x] Integration tests written (TestSubscription_AddedWhileRunning in appbuild: real loader, file written after the scheduler is built)
- [x] Happy path implemented (WatchConfig, reloadConfig on every tick)
- [x] Edge cases from planning handled (broken file keeps running tasks, logged once; broken subscriptions still apply schedules; unchanged config not republished; prune skipped on an incomplete config)
- [x] Error handling in place (reload errors logged once per distinct message and on recovery)

## Test Quality

- [x] Using fixture builders or factories for test data (newTestScheduler, scheduleYAML, subscriptionsYAML)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (appbuild e2e test drives the real loader and poll)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `go test -race ./internal/scheduler/` and `go test
./internal/appbuild/ -run TestSubscription_` pass. golangci-lint is clean on
scheduler, cli and rela-desktop. The desktop TestChromeStyle failure predates
the change (no built SPA locally).

## Quality

- [x] Code follows project patterns (atomic.Pointer publish, one load per tick)
- [x] Checked for DRY opportunities (duplicate task lookup removed from subscriptions.go)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
