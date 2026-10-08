---
id: IMPL-8DXF8U
type: implementation-checklist
title: 'Implementation: Relation-conferred type@face grant denies explicitly addressed faces at the row gate'
started: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new code; the fix landed in #1753)
- [x] Integration tests written (test full flow, not just units)
- [x] ~~Happy path implemented~~ (N/A: fixed by #1753; this change adds the regression test)
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: test-only change)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the handler-level test covers both routes)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: the bare-id negative case is in the test)

**Verification Evidence:**
TestComments_FaceLimitedConferredGrant (internal/dataentry): under a
relation-conferred `ticket@draft` grant, entity and comment GET on
`TKT-001@draft` return 200 and the bare id returns 404. Run on cf0fe87e^ the
granted-face subtests fail with 404, the reported symptom.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: test-only change)
- [x] No debug code left behind
