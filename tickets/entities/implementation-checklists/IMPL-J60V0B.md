---
id: IMPL-J60V0B
type: implementation-checklist
title: 'Implementation: Detail panel goes stale after a relation edit elsewhere'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: TestRelationEndTypes (internal/dataentry/watcher_relation_test.go); EntityDetail.events.test.ts (burst debounced into one reload, reload during a steady stream, events for another type ignored)
- [x] Integration tests written (test full flow, not just units): EntityDetail.events.test.ts mounts the detail component with the event bus
- [x] Happy path implemented: a relation write elsewhere reloads the open detail panel
- [x] Edge cases from planning handled: pending autosave is not overwritten; bursts are debounced (250 ms) with a 1 s max wait
- [x] Error handling in place (errors surfaced, not swallowed): the reload uses the existing detail load path and its error handling

## Test Quality

- [x] Using fixture builders or factories for test data: shared mount helper and `emit()` helper in EntityDetail.events.test.ts
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: on the demo server the detail panel follows a drag on the board
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified: pending edits across a reload are covered by code (SectionEditForm merges through autoSave.mergeServerResponse) and review R1-10, not by a manual run

**Verification Evidence:**
- Demo: open the side panel, drag the card; the panel shows the new status without a reload.
- `go test ./internal/...` passes. Frontend vitest full suite passes: 3770 tests in 255 files. `vue-tsc` is clean.

## Quality

- [x] Code follows project patterns (check similar code): same entity:changed subscription as list and kanban views
- [x] Checked for DRY opportunities: relationEndTypes is one helper used by the pump
- [x] No security issues introduced: the event carries only the entity type, no data
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
