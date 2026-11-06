---
id: IMPL-WKVJRPV
type: implementation-checklist
title: 'Implementation: Relation move skips the manager''s relation meta gate'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestMoveRelation_FieldGateJudgesOrderKey)
- [x] Integration tests written (test full flow, not just units) (TestRelationPosition_FieldsGrantRealPolicy drives the position PATCH against a compiled acl policy)
- [x] Happy path implemented
- [x] Edge cases from planning handled (incoming side judges every other source tail; refusal writes nothing)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: existing seedTask and seedEntity helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no surface outside data entry constructs a position; the handler path is covered by the compiled-policy test)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (the manager test fails with the gate call removed)

**Verification Evidence:**
TestMoveRelation_FieldGateJudgesOrderKey failed on both refusal cases with the
new check disabled ("move succeeded; the gate refuses _order_out on TASK-1")
and passes with it.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (reuses checkRelationMetaWrite)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
