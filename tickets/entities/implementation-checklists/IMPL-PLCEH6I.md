---
id: IMPL-PLCEH6I
type: implementation-checklist
title: 'Implementation: Manual list order'
started: "2026-11-24"
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
Built rela-server and ran it on a temp project with a `todo` list set to
`order: manual`, as RELA_DATAENTRY_USER=alice. POST /api/v1/_list_order moved
TODO-003 before TODO-001 (204); the list read with manual_order=1 returned
TODO-003, TODO-001, TODO-002; without the flag it kept the title sort. Without
a user the move answered 403, so the read now reports manual_order only for a
user. Go tests: TestManualOrder_* (move, errors, hidden row, filter, grouped)
and listorder unit tests. Vitest: useManualReorder.test.ts.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
