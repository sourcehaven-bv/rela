---
id: IMPL-JKS0X
type: implementation-checklist
title: 'Implementation: Detail-view list section items are not clickable (no href, broken router push)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code

  - `frontend/src/utils/entityRoute.test.ts` (5 cases): cellLink wins,
detail_view path, /entity/:type/:id fallback, empty type → empty string,
cellLink wins even with empty type.
  - `internal/migration/detail_view_to_entity_views_test.go`: Detect
cases (8), Apply cases (4), idempotency, placement-after-lists, in-tree-configs
guard.
  - `internal/dataentryconfig/validate_test.go`: 4 new cases for
`entity_views` validation (valid, absent, unknown type, unknown detail_view).

- [x] Integration tests written (test full flow, not just units)

  - `frontend/src/views/CustomView.test.ts` (3 cases): mounts CustomView
with vue-router and stubbed fetchView, asserts rendered `<a>` href
    + click triggers router.push with correct path + return_to query.

- [x] Happy path implemented

  - Backend Go config + validation
  - Migration `detail-view-to-entity-views`
  - Migration applied to in-tree configs (`tickets/data-entry.yaml`,
`prototypes/data-entry/project/data-entry.yaml`)
  - V1Config JSON response includes `entity_views`
  - SPA store loads `entity_views`; `getEntityDetailView` getter
  - Helper `entityDetailHref` in `frontend/src/utils/entityRoute.ts`
  - CustomView.vue list display = real `<a :href @click.prevent>` with
`:focus-visible` outline; cards/content cards/table cells updated to pass entity
through helper
  - EntityList.vue migrated to the same helper (column-link still wins)
  - Navigation passes `?return_to=/view/<viewId>/<entityId>` for back-button

- [x] Edge cases from planning handled

  - Empty `entity.type` → helper returns empty string; template guards
with `v-if="entityHref(entity)"` so we never emit `<a href="/entity//id">`.
  - Migration conflict groups → skipped, Detect() reports false on
second pass (idempotent).
  - Pre-existing `entity_views:` block → migration merges into it
rather than overwriting.

- [x] Error handling in place (errors surfaced, not swallowed)

  - Migration fails loudly if `entity_views:` exists but isn't a
mapping (rejects malformed hand-edited config).
  - Validate rejects unknown view references in `entity_views.<type>.detail_view`.

## Test Quality

- [x] Using fixture builders or factories for test data

  - Tests use small inline YAML / object literals. Fixtures only
contain values that matter for the assertion.

- [x] No hardcoded values in assertions when object is in scope

  - Component test asserts URL equality against constructed string
using the entity id from the test data.

- [x] Only specifying values that matter for the test

  - Test sections include only the fields the component reads.

- [x] Interpolated values constructed from objects, not hardcoded

  - return_to query asserted using template-literal-style construction.

- [x] Property comparisons use original object, not hardcoded strings

  - href asserted from id/type, not against a literal.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Started `rela-server --project tickets`. Loaded
`/view/feature_detail/FEAT-001`. The "Required Concepts" section rendered two
`<a class="list-link">` items. DOM check via puppeteer:

  ```text
  count: 2
  href: /view/concept_detail/cli-flags
  href: /view/concept_detail/data-entry-server
  ```

Both targets used the configured `entity_views.concept.detail_view`.

- Clicked the first link. URL changed to
`/view/concept_detail/cli-flags?return_to=/view/feature_detail/FEAT-001`, page
rendered as "Concept: CLI Flag Handling". Back button at the top-left was an `<a
href="/view/feature_detail/FEAT-001">← Back</a>` using the `return_to` query.

- Verified `:focus-visible` rule is compiled into the SPA bundle
(scoped `.list-link[data-v-…]:focus-visible`). Programmatic `.focus()` doesn't
trigger `:focus-visible` (browser behavior; only keyboard navigation does, by
design).

- API config response (`GET /api/v1/_config`) now includes
`entity_views` with concept/feature/future-concept/idea bindings.

## Quality

- [x] Code follows project patterns (check similar code)

  - Migration follows the existing `internal/migration/*` pattern
(init/Register, Detect/Apply, file-type-aware).
  - Helper is a pure function in `frontend/src/utils/`, alongside
`format.ts` / `markdown.ts`.
  - Validate function added next to existing `validateLists` etc.

- [x] No security issues introduced

  - No new user input paths; entity_views is YAML config validated at
load time. Helper produces SPA-internal paths, not raw URLs.

- [x] No silent failures (errors logged AND returned)

  - Helper returns empty string for missing type → template skips
rendering; not silent — visible by absence.
  - Migration errors propagate.

- [x] No debug code left behind

  - No console.log, no TODO comments beyond the documented future-direction
one in `entityRoute.ts` (server-side resolution).
