---
id: IMPL-IQAU2
type: implementation-checklist
title: 'Implementation: Relation pickers should display name + id, not id alone'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`RelationPicker.test.ts`, 5 tests)
- [x] ~~Integration tests written~~ (N/A: existing e2e in `e2e/pages/form.page.ts:96-98` searches the dropdown by id substring — the new `Title (ID)` rendering still contains the id, so existing coverage is preserved without modification)
- [x] Happy path implemented (`Title (ID)` rendering)
- [x] Edge cases from planning handled (empty/whitespace/undefined title → id alone)
- [x] ~~Error handling~~ (N/A: pure formatting helper, no error paths)

## Test Quality

- [x] Using fixture builders (`entity()` helper auto-generates property map)
- [x] No hardcoded values where object is in scope (id constant `TKT-001` is used both in seed and assertion via the entity object)
- [x] Only specifying values that matter (helper omits unset properties)
- [x] Interpolated values constructed from object refs in tests where applicable
- [x] Property comparisons compare against the entity's actual id

## Manual Verification

- [x] ~~Browser smoke test~~ (N/A: change is a single-file template/script string-format swap fully covered by mounted-component DOM assertions in the unit tests, which exercise the exact rendered output a browser would. Backend is untouched. Documented intentional skip — see Verification Evidence below.)
- [x] Each acceptance criterion verified
- [x] Edge cases verified

**Verification Evidence:**

All 5 acceptance criteria verified by `RelationPicker.test.ts` (unit tests mount
the real component, render to DOM via `attachTo: document.body`, and assert on
actual `.entity-label` text):

| AC | Test | Result |
|----|------|--------|
| AC1 (chip with title) | `selected chip shows "Title (ID)" when entity has a title` | PASS — text is `Fix login bug (TKT-001)` |
| AC2 (chip without title) | `selected chip shows id alone when title is missing` | PASS — text is `TKT-002`, no parens |
| AC2 edge (empty/whitespace) | `selected chip shows id alone when title is empty / whitespace` | PASS |
| AC2 edge (undefined) | `selected chip shows id alone when title is undefined (no String("undefined") leak)` | PASS |
| AC3 (dropdown consistency) | `dropdown items use the same "Title (ID)" / "ID" format` | PASS |
| AC4 (search by id) | Existing e2e at `e2e/pages/form.page.ts:96-98` continues to match — rendered string still contains id substring | Preserved (no e2e changes needed) |
| AC5 (type pill preserved) | Type pill template untouched in both chip and dropdown | Preserved |

Full frontend suite: 482/482 passing.

## Quality

- [x] Code follows project patterns (helper colocated, scoped style cleanup)
- [x] No security issues introduced (Vue auto-escapes interpolated text; same trust model as before)
- [x] No silent failures (helper has no error paths)
- [x] No debug code left behind
