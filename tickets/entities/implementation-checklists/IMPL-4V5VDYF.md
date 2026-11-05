---
id: IMPL-4V5VDYF
type: implementation-checklist
title: 'Implementation: Conflict resolve writes without a read gate'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (handler tests over an app with the ACL wired into the entity manager)
- [x] Happy path implemented
- [x] Edge cases from planning handled (relation files, missing files, id mismatch, visible: fields)
- [x] Error handling in place (gate errors answer writeGateError, never fold into hidden)

## Test Quality

- [x] Using fixture builders or factories for test data (newConflictACLApp, conflictReadGateApp)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: handler-level tests cover the HTTP behaviour; conflicted files need a git merge to produce)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (hidden detail body compared to a missing file's)

**Verification Evidence:** TestV1Conflicts_ReadGate,
TestV1ConflictResolve_HiddenRowIs404, TestV1ConflictResolve_IDMustMatchFile and
TestV1ConflictDetail_RedactsHiddenProperties fail on develop and pass with the
fix. go test ./internal/dataentry passes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
