---
id: IMPL-OBV0P2
type: implementation-checklist
title: 'Implementation: Attachment upload and delete ignore the fields: write policy'
started: "2026-10-09"
completed: "2026-10-09"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: the test drives the real HTTP handler with a real ACL and field resolver; a manual run adds nothing)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: same reason; upload and delete are both covered by the test)

**Verification Evidence:** TestAttachmentWrite_HonorsReadOnlyField failed before
the fix (upload returned 200 and replaced the file) and passes after; go test
./internal/dataentry passes and golangci-lint reports 0 issues.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
