---
id: IMPL-E6C428S
type: implementation-checklist
title: 'Implementation: Property test: two clients autosaving one entity'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (useAutoSave.twoClients.prop.test.ts: property over two tabs plus two permanent fault tests and a diff3 check)
- [x] Integration tests written (test full flow, not just units) (the property drives two real useAutoSave instances through the verified fake API)
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: the orders need two tabs and exact answer timing; covered by the scheduler-driven property test)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: same reason; each edge case is a fault or command in the property test)

**Verification Evidence:**

- `PROP_RUNS=2000` passes; 300 runs take about 2 s.
- Faults: mergeProperty or mergeText writing ours fails the property (permanent tests).
- One-off mutations of the base rules each fail it (RR-5OG51BQ).
- Full suite: 365 files, 5480 tests pass; vue-tsc clean; eslint 0 errors.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
