---
id: BUGA-39RRSC
type: bug-analysis-checklist
title: 'Analysis: Relation-picker autosave aborts with "unknown types" when the linked entity is outside the 100-candidate window'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced as a Playwright e2e test,
`e2e/tests/relation-picker-large-candidate-set.spec.ts`. It fails on `develop`
with exactly the reported symptom: the chip renders, the toast reads *"Some
related entities have unknown types; relation changes were not saved. Reload the
form and try again."* (confirmed verbatim via a temporary page-text probe), no
PATCH is sent, and the server still holds `["FEAT-001", "FEAT-104"]` — the newly
picked `FEAT-002` was never written.

Minimal conditions: a legacy (non-cards) `RelationPicker` on an **edit** form
(autosave path), where the relation's target type has **more than 100** entities
and the entity's **existing** link points at one outside the first page. Adding
any second value then aborts the whole form's relations autosave.

The trigger is purely the size of the target-type collection, not multi-user
activity. The original report's SSE hypothesis is not needed to explain it.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

`RelationPicker.buildOutgoingTypes()` derives the id → type map solely from its
local `candidates` array, and `loadCandidates()` fills that array with a single
page (`fetchList(targetType, { per_page: 100 })`). An already-linked entity past
the page boundary therefore has no resolvable type, `reshapeLegacyToModern`
returns `null`, and `buildAutoSaveRelationsBody` aborts the entire relations
autosave rather than the one affected field.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Fix**: page the candidate collection in `loadCandidates()` via the existing
`listAllEntities()` helper, the same helper `KanbanView` and `CalendarView`
already use for exactly this reason (BUG-5OAQUG).

**Regression test**: the e2e spec above, which is red before the fix.

**Related areas**: `grep "per_page: 100"` across `frontend/src` returns only
`RelationPicker.vue:161` and the `listAllEntities` implementation itself, so
this was the last single-page consumer of a collection it renders in full.
