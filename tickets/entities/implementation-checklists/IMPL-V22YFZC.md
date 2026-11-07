---
id: IMPL-V22YFZC
type: implementation-checklist
title: 'Implementation: Erase deleted-entity state from automation job payloads'
started: "2026-11-07"
completed: "2026-11-07"
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
- `TestDeletedTrigger_EndToEnd/no_acl,_queued` runs a soft delete through the
real memory queue and state store; the script still sees the last state.
- `TestDeletedState_*` pass; with the erase calls removed, the completed,
dropped, final-failure and soft-delete-purge cases fail.
- `TestPostgresQueue_RetentionSweep`, `_RetentionSweepBacklog` (4000 rows over
several batches) and `_CloseLifecycle` pass against local PostgreSQL.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
