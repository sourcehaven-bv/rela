---
id: IMPL-385KRJ
type: implementation-checklist
title: 'Implementation: Faced types: attachment upload/download and entity export 404 on ID@face; file bytes shared across faces'
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

- Handler tests (`internal/dataentry/attachment_face_test.go`): upload stays on
  its face, delete counts references, face delete drops unreferenced bytes, ACL
  on `type@face`, ordinary file write is 422, history restore keeps live file
  values, export addresses its face.
- Service tests (`internal/attachment/faced_test.go`, `-race`): a `fields: all`
  copy shares bytes; a copy racing a detach keeps references and bytes
  together (fails with the copy lock removed); a bare id is refused naming the
  faces.
- MCP (`internal/mcp/tools_attachment_test.go` `TestAttachments_PerFace`) and
  entitymanager write-rule tests (`internal/entitymanager/attachments_test.go`).
- E2E: the three BUG-CTUW2N specs in `e2e/tests/faces-backlog.spec.ts` flipped
  and pass; `attachments-create.spec.ts` passes 3x (its upload wait helper now
  waits for every PUT).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
