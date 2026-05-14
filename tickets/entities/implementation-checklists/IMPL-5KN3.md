---
id: IMPL-5KN3
type: implementation-checklist
title: 'Implementation: Define entitymanager.Manager (real implementation, not adapter)'
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: library/internal refactor — no end-user surface; verified via Go race-enabled test suite + arch-lint + coverage)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `just ci` green locally (race tests, lint, arch-lint, coverage, docs).
- `TestCreate_PropagatesNonConflictStoreError` pins the upsert error-propagation regression (cranky C1).
- `TestCreate_AutomationCreatesRelatedEntity` exercises the cascade dispatch from Manager.
- `TestCreate_WritesTwiceWithAutomationProperty` pins the "two writes when automation sets a property" invariant.
- `TestRename_AppliesAndRewritesRelations` plus `TestRename_SelfReferentialCountsTwice` lock down the extracted rename orchestration against workspace's pre-extraction count semantic.
- `TestUpdate_NotFoundReturnsTypedError`, `TestDelete_NotFoundReturnsTypedError`, `TestCreateRelation_DuplicateRejectedTyped` etc. verify typed-error contract.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
