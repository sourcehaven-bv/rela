---
id: BUGA-GI7OD1F
type: bug-analysis-checklist
title: 'Analysis: SPA edit form saves an edit onto a different entity after in-form navigation'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (`frontend/src/components/forms/DynamicForm.entitySwitch.test.ts`: 3 failing tests)
- [x] Minimal reproduction steps documented: open the edit form of X, change a field, within 800 ms navigate to the edit form of Y with the same form id (side-panel link, back/forward). The PATCH goes to Y; the form keeps X's values under Y's id.
- [x] Environment/conditions noted: any backend; SPA only. Needs a same-route param change (`/form/:id/:entityId`) and, for the edit to survive the 412, Y's field value equal to X's merge base.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined: fixed `{type, id}` target in `useAutoSave`; key `DynamicForm` per entity in `FormView`; flush on `onBeforeRouteUpdate`; fail closed in `useAutoSave` when a snapshot or 412 refetch names another entity; move `EntityDetail` from the pin to the same model.
- [x] Regression test planned: `DynamicForm.entitySwitch.test.ts` plus a `useAutoSave` target-mismatch unit test (AM-autosave-target-fixed-per-edit).
- [x] Related areas checked for similar issues: `EntityDetail` (late-bound, guarded by `pinEntityForFlush`), `SectionEditForm` (frozen owner, safe), kanban/calendar/gantt/list/page editing/inline relation writes (target read at action time, no debounce, safe).
