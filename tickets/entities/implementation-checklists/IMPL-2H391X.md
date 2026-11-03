---
id: IMPL-2H391X
type: implementation-checklist
title: 'Implementation: Copy option to keep named fields on the target face'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (metamodel copies_test.go keep cases; copydef_unmarshal_test.go)
- [x] Integration tests written (test full flow, not just units) (attachment faced_test.go TestFaced_CopyKeepLeavesTargetUpload runs CopyState with real attachments)
- [x] Happy path implemented
- [x] Edge cases from planning handled (new target face, repeated copy, keep without fields: all, undeclared name)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (newFacedFixture, copyFixture)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (via the CopyState integration test with real attachment bytes)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** A `fields: all` copy with `keep: [spec]` leaves the
target's own upload and its bytes downloadable after two copies, still copies
title, and leaves spec unset on a new target face. Load refuses keep without
fields: all and undeclared names.

## Quality

- [x] Code follows project patterns (check similar code) (keepTargetValues generalizes keepTargetExternalRefs)
- [x] Checked for DRY opportunities (external refs and keep share keepTargetValues)
- [x] No security issues introduced (keep only narrows what a copy writes)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
