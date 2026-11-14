---
id: IMPL-UIWDZ9A
type: implementation-checklist
title: 'Implementation: Remove unpaged list reads from request paths'
started: "2026-11-13"
completed: "2026-11-13"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: no user-facing change; verified through the manager, handler and binding tests)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `TestCreate_CostIsScaleFree`,
`TestGanttEdges_CostIsScaleFree` and `TestRoleHolders_CostIsScaleFree` pass; the
gantt test fails when the query is switched back to DirectionBoth.
`TestNoNewUnpagedReads` passes with the fixed entries removed. Race tests pass
on every touched package; postgres-tagged entitymanager, dataentry webhook and
pgstore suites run against a local Postgres 18.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
