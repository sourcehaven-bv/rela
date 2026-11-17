---
id: IMPL-JSN8G0B
type: implementation-checklist
title: 'Implementation: Store query contract: or, not and ordered comparisons'
started: "2026-11-17"
completed: "2026-11-17"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: no UI or config change; the served paths are covered end to end by the MCP list_entities and data-entry list comparison tests, which serve each condition through the real handler and compare with the Go path)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `storetest.RunGraphCondTests` and `RunGraphDifferential` pass on memstore, fsstore, sqlitestore and pgstore (local PostgreSQL 18).
- `TestLowerCondition_AgreesWithEvaluator`: 300 random conditions over awkward values, Go and store agree.
- `TestListEntities_ConditionMatchesGoPath` (MCP) and `TestListPushdown_ConditionMatchesGoPath` / `TestListPushdown_ScopeMatchesGoPath` (data entry) serve or, not and ordered conditions from the store and match the Go path for every principal.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
