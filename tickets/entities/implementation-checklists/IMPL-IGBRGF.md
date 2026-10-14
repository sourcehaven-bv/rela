---
id: IMPL-IGBRGF
type: implementation-checklist
title: 'Implementation: Pile writes fail in a non-default world; Create pile button is off-centre'
started: "2026-10-13"
completed: "2026-10-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (usePiles world-bound write test; RlButton Pending play test)
- [x] Integration tests written (test full flow, not just units) (TestPiles_WriteThroughWorldMiddleware runs the piles handler behind attachWorld)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a face the default world hides; a pending-label button without a label keeps the old slot)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (newPilesApp, mountPiles)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (partly: see evidence)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Local rela-server on prototypes/worlds/project: `POST
/api/v1/_piles?world=editorial` returns 422 world_read_only, as in the report.
The fixed world-less request reaches the piles handler, which needs a signed-in
owner the local server lacks; the accepted path is covered by
TestPiles_WriteThroughWorldMiddleware. Both regression tests were checked to
fail against the old code (usePiles: 3 failures; RlButton Pending story:
centring assertion fails). Full frontend suite: 4193 tests pass; typecheck and
lint clean; golangci-lint clean on internal/dataentry.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (spinnerSlotStyle shared by both slot sites)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
