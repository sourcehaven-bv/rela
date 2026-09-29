---
id: IMPL-GQQIJK
type: implementation-checklist
title: 'Implementation: sqlite swept versions lose copy provenance (no origin columns on live rows)'
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

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `storetest.RunSweepOriginTests` (three cases) fails on sqlite without the sweep change and passes on sqlite and postgres.
- `TestSQLiteSweptCopyNamesItsSource`: a real `CopyState` through the assembled sqlite app yields a swept version with Kind copy, the definition name and source label `PAGE-…@live`; fails without the fix (`expected "copy", actual ""`).
- `TestMigrateToOriginColumns`: v8 database gains the columns, existing rows are NULL, re-running the step is safe.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
