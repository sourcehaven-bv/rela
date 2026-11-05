---
id: IMPL-H1H5JRM
type: implementation-checklist
title: 'Implementation: Live view misses a change reverted before the next round'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestLiveSub_ServedRecords, TestLiveSub_ServedStreakStopsLoop)
- [x] Integration tests written (test full flow, not just units) (live_served_test.go drives real GETs, leases and writes through the router with a manual round clock)
- [x] Happy path implemented
- [x] Edge cases from planning handled (records during a round, records after a commit, syncing subs, 304, failed writes, other principals, hidden writes, replay that never matches)
- [x] Error handling in place (errors surfaced, not swallowed) (a failed write skips the record; no new error paths)

## Test Quality

- [x] Using fixture builders or factories for test data (ticket1, feature, seedEntity, newLeaseEnvWith, manualClock)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (e2e/tests/presence.spec.ts, the spec the bug was found in)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (unit and manual-clock tests above)

**Verification Evidence:** TestLive_LeaseClaimedAndReleasedBetweenRoundsFrames
and TestLive_WriteRevertedBetweenRoundsFrames failed on develop on every run and
pass with the fix. `go test -race ./internal/dataentry/` passes. `npx playwright
test tests/presence.spec.ts --repeat-each=30 --workers=12`: 60 passed.

## Quality

- [x] Code follows project patterns (check similar code) (pending set, round generation and hashWriter reused)
- [x] Checked for DRY opportunities (liveHash shared by replays and served responses)
- [x] No security issues introduced (rela-security-reviewer: two findings, both addressed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
