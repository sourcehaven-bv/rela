---
id: IMPL-HQSLXM
type: implementation-checklist
title: 'Implementation: Collapsed background-job trigger resets the hop count'
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestAutomationJobs_UnhandledTriggerKeepsHops`, `TestTokenHops`.
- [x] Integration tests written: `TestAutomationJobs_QueuedChainStops` runs two
jobs that trigger each other on a real memory queue and asserts hops 1..8.
- [x] Happy path implemented
- [x] Edge cases from planning handled: legacy tokens without a count, a
later low-hop trigger, a handled token.
- [x] Error handling in place: `recordTrigger` returns KV errors to `EnqueueScript`.

## Test Quality

- [x] Using fixture builders or factories for test data (`newTestJobs`, `startedQueue`)
- [x] No hardcoded values in assertions when object is in scope (expected
sequence built from `maxAutomationJobHops`)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] ~~Property comparisons use original object~~ (N/A: no entity properties compared)

## Manual Verification

- [x] Feature manually tested end-to-end: stress run under the race detector.
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Before the fix, 5 of 1,200 runs of
`TestAutomationJobs_QueuedChainStops` failed (`-race -count=400 -cpu=1,2,8`),
with run logs such as `a@3 b@4 a@3 b@4`. After it, 1,800 runs and 600 runs of
all `TestAutomationJobs*` pass. `internal/appbuild` passes on the fs and sqlite
builds.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities: the token read/write lives in `recordTrigger`.
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
