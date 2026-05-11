---
id: IMPL-7OOI
type: implementation-checklist
title: 'Implementation: Remove +Add / Link Existing buttons from data-entry view widgets'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written~~ (N/A: pure deletion; AC3 is covered by `TestV1Views_NoAddOrLinkInfoOnSections` which is already a handler-level integration test that decodes the actual response body)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (`newTestAppV1`, `seedEntity`, `seedRelation`)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test (the new test only sets `title` props on entities; everything else uses helpers)
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- AC1 (no Add/Link buttons in EntityDetail): grep confirms `EntityDetail.vue` has zero remaining references to `addInfo`, `linkInfo`, `+ Add`, `Link Existing`, `navigateToCreate`, `openLinkExisting`, or `LinkExistingModal`. Vue typecheck passes (`npm run typecheck` → no errors).
- AC2 (side panel unchanged): `SidePanel.vue` and `V1SidePanelSection` were not touched. The `resolveSectionButtonsWithTraverse` call at the side-panel handler (`api_v1.go` line 1758 — moved up after the view-handler call deletion) remains. Backend tests pass: `go test ./internal/dataentry/...` → ok.
- AC3 (view JSON has no `addInfo`/`linkInfo`): new test `TestV1Views_NoAddOrLinkInfoOnSections` configures a `cards`-display section over an `implements` traversal — the exact shape that previously emitted both fields — and asserts both keys are absent in the marshaled JSON. Decodes through `map[string]json.RawMessage` so a future re-introduction with a renamed Go field would still get caught.
- AC4 (side-panel JSON unchanged): no code path touched; existing side-panel tests in `api_v1_test.go` still pass under `go test ./internal/dataentry/...`.
- AC5 (per-row Edit pencils): `navigateToEdit` and the `<button v-if="row.editFormId">` template entries in EntityDetail are untouched. Visually verified via `grep "navigateToEdit"` showing the function and its template usages still present.
- AC6 (build hygiene): `just test` ✓ all packages pass, `just lint` ✓ 0 issues, `just arch-lint` ✓ no warnings, `just coverage-check` ✓ 75.9% total coverage, `npm run typecheck` ✓ clean, `npm run test:run` ✓ 650/650 tests pass, `npm run lint` ✓ 0 errors (74 pre-existing warnings, all in `stress/` outside scope).

## Quality

- [x] Code follows project patterns (deletion follows the same pattern as TKT-9QNHN's read-only-view philosophy; no new abstractions introduced)
- [x] No security issues introduced (the change reduces wire surface and removes UI affordances; underlying mutation APIs are unchanged and still authenticated where they were)
- [x] No silent failures (no error-handling paths added or removed; the deleted call had no error return)
- [x] No debug code left behind
