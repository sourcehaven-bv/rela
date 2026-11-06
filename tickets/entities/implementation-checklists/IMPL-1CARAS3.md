---
id: IMPL-1CARAS3
type: implementation-checklist
title: 'Implementation: Relation fields: grants leave unnamed meta fields writable'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestRelationMeta_FieldsGrantIsAnAllowlist, TestWriteGate_RelationMetaFollowsTheAllowlist, TestRelationMeta_FieldGateJudgesCallerMeta)
- [x] Integration tests written (test full flow, not just units) (TestV1RelationMeta_FieldsGrantIsAnAllowlist drives PATCH, unified PATCH and POST through a compiled policy; conflict resolve cases in TestV1ConflictResolve_FieldGateRealPolicy)
- [x] Happy path implemented
- [x] Edge cases from planning handled (undeclared keys, role union, failing when, order properties, unchanged keys, conflict resolve of ungoverned keys)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (newUnionWorld, newConflictACLApp, newPatchManager)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (handler tests against a compiled acl.yaml policy)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** The new tests failed on the pre-fix code (8 verdict
failures; 200 instead of 403 for `note` and `extra`) and pass now. go test on
affordances, entitymanager, dataentry, store, mcp, appbuild, cli passes.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors fieldVerdicts role union and the entity FieldWriteGate)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (ChangedRelationMeta shared by dataentry and
entitymanager; MetaWritable on the verdict)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
