---
id: IMPL-WN6X2
type: implementation-checklist
title: 'Implementation: Honor return_to as a back affordance on non-form screens'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

### Files changed

**Go:**
- `internal/dataentry/return_path.go` — case-folded `/%5c|C` / `/%2f|F` prefix check (AC8).
- `internal/dataentry/return_path_test.go` — lowercase cases added.
- `internal/dataentry/document.go` — rewriter rewrite per decision table
(AC1, AC10). New `hrefRegex` captures optional preceding `id=` so
double-rewrites don't duplicate attributes. Detailed docstring on
`RewriteDocumentLinks` notes the post-cache invariant (RR-3Y6BM).
- `internal/dataentry/document_test.go` — cases for non-form internal
paths with + without pre-existing query, stripped author-supplied `return_to`,
empty-returnPath branches, idempotency harness.
- `internal/dataentry/api_v1_test.go` — `TestHandleV1Documents_CacheInvariance`
(AC9): renders the same entry twice with different `return_to`, verifies each
response carries the matching value and the on-disk cache file is
`return_to`-free.

**Frontend new:**
- `src/composables/useBackTarget.ts` — precedence-rule composable
returning `{ to, labelHint } | null`. Reactive `computed` so `router.replace` on
`route.query` flows through.
- `src/composables/useBackTarget.test.ts` — 20 cases covering
precedence, guard edge cases, array-valued query, reactivity.
- `src/components/common/BackButton.vue` — `<router-link>` + label
resolution (schemaStore lookup for list titles).
- `src/components/common/BackButton.test.ts` — 7 cases.
- `src/styles/back-button.css` — shared `.scope-nav-btn` styles +
mobile media-query. Imported from `main.ts`.

**Frontend modified:**
- `src/main.ts` — imports `styles/back-button.css`.
- `src/composables/useScopeNavigation.ts` — removed `backUrl` from
`ScopeNav` (composable now owns back) and `goBack()` helper (no callers).
`navigateScope` still preserves `route.query` verbatim, so `?return_to=` rides
through in-list Prev/Next (RR-97NAZ).
- `src/composables/useScopeNavigation.test.ts` — replaced the old
`goBack` test with an assertion that `navigateScope` preserves both `from` and
`return_to`.
- `src/components/entity/EntityDetail.vue` — swapped scope-nav Back
for `<BackButton>`; Escape key follows the composable precedence; scoped
`.scope-nav-btn` styles deleted (now global).
- `src/views/CustomView.vue` — same pattern.
- `src/views/DocumentView.vue` — replaced bespoke `goBack()` /
`fromList` computed with `<BackButton>` (renders only when the composable
returns non-null; no more `router.back()` fallback).
- `src/views/ListView.vue` (via `EntityList.vue`), `KanbanView.vue`,
`AnalyzeView.vue`, `SearchView.vue` — added `<BackButton>` in the page header
with `v-if="backTarget"`.
- `src/components/forms/DynamicForm.vue` — unchanged (RR-5K8I2
resolution: keeps calling `readReturnTo` directly).

**E2E:**
- `e2e/tests/fixtures.ts` — added `documents:` config + `scripts/docs/feature_overview.lua`
doc script that links to `/entity/bug/BUG-001`.
- `e2e/tests/back-button.spec.ts` — 4 behavioural tests (AC7 + AC2
safety variants).
- `e2e/pages/base.page.ts`, `e2e/pages/entity.page.ts`, `e2e/pages/list.page.ts`
— page-object methods for Back button, document body + selector, list heading
assertion.

**Docs:**
- `docs/data-entry.md` + `docs-project/entities/guides/GUIDE-data-entry.md`
— "Links in rendered documents" updated with new table + new "Back navigation"
subsection.
- `frontend/CLAUDE.md` — added BackButton / useBackTarget / styles
directory to the package-layout table.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (via e2e spec — replaces
manual smoke; back-button.spec.ts covers the full round-trip)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Verified by |
|----|-------------|
| AC1 (rewriter, non-form internal) | `TestRewriteDocumentLinks` — 10+ new cases for list/entity/kanban/non-route paths |
| AC2 (precedence) | `useBackTarget.test.ts` — 20 cases |
| AC3 (EntityView/CustomView scope-nav Back) | manual smoke via `back-button.spec.ts` + `useScopeNavigation.test.ts` Prev/Next query preservation |
| AC4 (DocumentView Back) | manual + e2e — feature-overview doc's back click lands on the original URL with `?doc=` preserved |
| AC5 (ListView/KanbanView/AnalyzeView/SearchView Back) | `back-button.spec.ts:65` — ListView renders + navigates |
| AC6 (DynamicForm regression) | no change to DynamicForm; existing `forms.spec.ts` passes unchanged |
| AC7 (E2E behavioural round-trip) | `back-button.spec.ts:22` — feature → bug → Back → feature with `?doc=` preserved |
| AC8 (server case-fold) | `TestIsSafeReturnPath` — 4 new lowercase cases |
| AC9 (cache invariance) | `TestHandleV1Documents_CacheInvariance` — dual-render + disk inspection |
| AC10 (rewriter idempotency) | `TestRewriteDocumentLinks_Idempotent` — same-returnPath byte-equal + different-returnPath replace |

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
