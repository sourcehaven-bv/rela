---
id: IMPL-IUCB8A3
type: implementation-checklist
title: 'Implementation: File-store conflict-block refusal answers HTTP clients with a generic error'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestWriteInternalError_ConflictBlockIs422)
- [x] Integration tests written (test full flow, not just units) (TestWrite_ConflictBlockBodyIs422 over a real file store)
- [x] Happy path implemented
- [x] Edge cases from planning handled (relation content after an entity write names the partial save; detail never echoes the body)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: existing seedEntity helper)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (CLI `rela create` and `rela update` with a fenced conflict block print the sentinel message and store nothing)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
Before the fix, relation content in a unified PATCH answered 500
`relation_write_failed` and entity create and PATCH answered 422
`validation_failed`. After it, all answer 422 `conflict_block`. The CLI prints
"write entity: the content holds a git conflict block, ...". MCP returns the
same error text.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (one writeConflictBlock helper)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
