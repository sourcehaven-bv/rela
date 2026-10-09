---
id: IMPL-1QV1JG
type: implementation-checklist
title: 'Implementation: SQLite force-live purge was re-captured by the sweep'
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: store-level fix; covered by conformance tests)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (old column order restored: the new case fails; zero-face, faced and relation cases pass)

**Verification Evidence:**
SweepBacklog conformance on sqlite passes; lint clean for sqlite/postgres tags (pgstore findings pre-existing, fixed by #1819).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
