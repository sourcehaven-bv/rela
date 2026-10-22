---
id: IMPL-NU90WD
type: implementation-checklist
title: 'Implementation: Per-face fields: policy'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (faceblocks_test.go: verdicts and load errors; aclaudit A14/B4/B5 face cases)
- [x] Integration tests written (test full flow, not just units) (dataentry attachment_face_fields_test.go: upload, PATCH on both faces)
- [x] Happy path implemented
- [x] Edge cases from planning handled (unknown type, faceless type, unknown face, empty face, double @, face key in visible:/relations:)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (facedAttachmentApp, faceBlockMeta)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (via the HTTP-level dataentry test, which drives the real handlers and resolver)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (load errors via TestNew_FaceBlockKeyErrors)

**Verification Evidence:** Upload to TKT-1@published screenshot returns 200 with
`fields: {ticket@published: [screenshot]}`; PATCH title on the published face
returns 403; PATCH title on the draft face returns 200. go test ./internal/...
passes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (parseBlockKey shared by all four block kinds; rowGrants/typeGrants helpers)
- [x] No security issues introduced (a face block only narrows a role on its face; closed-world union unchanged; no runtime deny)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
