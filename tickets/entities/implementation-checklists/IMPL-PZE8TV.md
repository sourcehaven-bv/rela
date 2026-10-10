---
id: IMPL-PZE8TV
type: implementation-checklist
title: 'Implementation: Face move loses edges when a carry fails on fs/mem'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: internal recovery path; the tests drive the real migration runner over memstore with an injected store failure, which a manual run cannot produce)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: same reason; the clash and re-run cases are covered by tests)

**Verification Evidence:** TestMigrateFace_RerunRecoversAFailedEdgeCarry failed
before the fix (watched-by edge lost) and passes after.
TestMigrateFace_RefusesToCarryOverADifferentEdge covers the clash. go test
./internal/datamigration ./internal/cli and the sqlite-tagged store tests pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
