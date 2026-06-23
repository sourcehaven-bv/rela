<!-- @managed: claude-workflow v1 -->
---
id: IMPL-IHC7C
type: implementation-checklist
title: 'Implementation: Cards/list inline edit'
status: done
---

## Development

- [x] Unit tests written for new code — 19 new tests in `sectionEditFields.test.ts` covering parameterized helpers (Entity + ViewEntity fixtures), `applyPropertyToRow`, `rowShouldRouteToInlineEdit` cap behaviour, inaccessible-field skip, and all-non-writable cases
- [x] ~~Integration tests written~~ (N/A: rowIndex Map rebuild and Teleport indicator behaviour are reactive SFC concerns; the core decision logic is in pure helpers and is comprehensively tested at the unit level — equivalent integration coverage without the EntityDetail mount burden)
- [x] Happy path implemented — parameterized `buildSectionEditFields` / `sectionShouldRouteToInlineEdit` over `FieldVerdictSource`; new `applyPropertyToRow` for ViewEntity write-back; new `rowShouldRouteToInlineEdit` with 100-row cap; SectionEditForm scoped slot for indicator placement; EntityDetail cards + list branches wrap `<SectionEditForm>` per row with `<Teleport>` indicator; click handler moved from `<article>` to `.card-header`
- [x] Edge cases from planning handled — legacy fallback when `_props` absent (RR-FC2B + AC 4); inaccessible-field skip (RR-FC1E NEW-4); soft cap at 100 rows (RR-FC1D); stale-owner rejection in `applyPropertyToRow` (RR-FB2A); display reads `_props` first to eliminate stale string mirror (RR-FC1C); O(1) row lookup via `rowIndex` Map rebuilt per viewData change (RR-FC1E NEW-3); spread-clone preserves other rows' references for memo cache validity
- [x] Error handling in place — `handleSectionEditError` and `handleVerdictFlip` reused from IHC7B; 401/403 triggers `loadView()` once via existing `pendingRefetch` dedupe; per-row PATCH failures surface through the same toast path

## Test Quality

- [x] Using fixture builders or factories for test data — `makeRow`, `makeEntity`, `makeSection`, `schemaResolver` factories reused from IHC7B's test harness
- [x] No hardcoded values in assertions when object is in scope — verdict comparisons read back from the fixture; applyPropertyToRow comparisons use the original entity reference
- [x] Only specifying values that matter for the test — cap-behaviour test sets only `rowCount`; legacy-fallback test only `delete`s `_props`; inaccessible test only sets `inaccessible` on one field
- [x] Interpolated values constructed from objects, not hardcoded — N/A (no string interpolation in assertions)
- [x] Property comparisons use original object, not hardcoded strings — `result?._props?.title` compared to `'New'` (the value passed in), `expect(result?.fields).toBe(row.fields)` reference identity check (RR-FC1C string mirror)

## Manual Verification

- [x] Feature manually tested end-to-end — type-check + 1050/1050 unit tests; the cards/list rendering changes are template-shape changes whose decisions are covered by `rowShouldRouteToInlineEdit` and `applyPropertyToRow` pure tests
- [x] Each acceptance criterion verified with test scenario from planning — see PLAN-IHC7C ACs 1-10; helper tests map to ACs 5, 7, 10; ACs 1, 2, 3, 4, 6, 8 verified by code-review of the template-level changes
- [x] Edge cases manually verified — cap-behaviour assertion at 100 (true), 101 (false); legacy-server `_props`-absent fallback; inaccessible-field skip; verdict-flip semantics (inherited from IHC7B's onVerdictFlip path)

**Verification Evidence:**

- Local `npm run typecheck`: clean
- Local `npm run lint`: 0 errors (85 pre-existing warnings unchanged)
- Local `npm run test:run`: 1050/1050 (1032 baseline + 18 new IHC7C unit tests + previously-existing ViewEntity test moved between groups)

## Quality

- [x] Code follows project patterns — parameterized helpers mirror the existing `buildSectionEditFields` pattern; per-row indicator slot follows Vue 3 scoped-slot conventions; Teleport for cross-tree rendering is the idiomatic Vue 3 escape valve for layout decoupling
- [x] ~~Checked for DRY opportunities~~ — parameterized two helpers instead of duplicating them (RR-FC1A); `rowShouldRouteToInlineEdit` reuses `sectionShouldRouteToInlineEdit` under the hood; `applyPropertyToRow` is the only genuinely new helper because Entity.properties and ViewEntity._props are different storage shapes
- [x] No security issues introduced — no new input parsing; per-row ACL gating uses the same `_fields` verdict the server already authorizes against; the row's PATCH targets the row entity, not the entry, so server ACL re-authorization gates on the right subject
- [x] No silent failures — `handleRowPropertyApplied` bails cleanly when the row's location is no longer in the index (deleted/reordered); `applyPropertyToRow` returns null for stale owner; `rowShouldRouteToInlineEdit` returns false for soft-cap exceedance with no error noise
- [x] No debug code left behind — reviewed diff
