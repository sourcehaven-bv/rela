---
id: IMPL-MHKZ4
type: implementation-checklist
title: 'Implementation: Merge EntityDetail and CustomView into a single config-driven detail screen'
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

Three commits on `develop`:

1. `61d81be` — backend handler re-keying + default-view synthesizer
2. `977bae8` — frontend EntityDetail/CustomView merge
3. `0704be3` — migration + in-tree config rewrites

**Backend (commit 61d81be):**
- New `internal/dataentry/default_view.go` (synthesizer + tests, 9 fixture tests).
- `internal/dataentry/api_v1.go` — `handleV1Views` rewritten to parse `/_views/{entityType}/{entityId}` and dispatch to explicit-vs-default. 6 handler tests added.
- `internal/dataentryconfig/validate.go` — duplicate-`entry.type` rejection at config-load time. Test added.
- `just test`, `just lint`, `just arch-lint` all green.

**Frontend (commit 977bae8):**
- `EntityDetail.vue` rewritten to consume `fetchView` and render the section-based response. CustomView's section rendering branches preserved (properties / content / cards / list / table / content-cards); EntityDetail's affordances preserved (Edit + Delete, mobile responsive header, command modal, documents panel, mermaid + interactive checkboxes in body content).
- `CustomView.vue` deleted.
- `/view/:id/:entityId` route deleted.
- `fetchView` signature changed to `(entityType, entityId)`.
- `EntityList` row-click no longer follows `detail_view` (always entity route now).
- `ListConfig.detail_view` field removed from frontend types.
- `npm run typecheck`: clean. `npm run lint`: 0 errors (73 pre-existing warnings unrelated to this work). `npm run test:run`: 601 passed.

**Manual verification (puppeteer-driven):**
- `/entity/concept/data-entry-ui` (configured-view path) — full screenshot captured. Sections rendered as configured: properties → content (none) → "Governed By Decisions" (cards, empty) → "Required By Features" (list with status/priority badges) → "Work Items" (table) → "Test Coverage" (list, empty) → "Future Extensions" (cards, empty). Edit + Delete actions visible.
- `/entity/ticket/TKT-J5BET` (default-view path — synthesized config) — full screenshot captured. Properties section auto-rendered with TITLE / KIND / PRIORITY / EFFORT / TAGS / STATUS as styled badges; content body rendered with full markdown; jump bar listed all 16 relation type names; sections show outgoing relations (affects, has-implementation, has-planning, implements) populated and remaining relations as empty-state.
- Scope-nav verified: navigated `/list/all_tickets` → first row → entity-detail at `[1/25]`. Clicked Next → moved to next ticket, `[2/25]`.
- Old `/_views/{viewId}/{entityId}` API call returns 404 post-migration; new `/_views/{entityType}/{entityId}` returns 200 for both default and configured paths.

**Migration (commit 0704be3):**
- `internal/migration/views_by_entity_type.go` — new migration. Re-keys `views:` from view-id to `entry.type`; strips `detail_view:` from list configs; errors with both view IDs named when multiple views target the same entity type.
- 7 migration tests covering Detect/Apply/idempotency/error-on-duplicate/preserve-malformed.
- Applied to `tickets/data-entry.yaml`: re-keyed 4 views (`idea_detail`/`future_concept_detail`/`feature_detail`/`concept_detail` → `idea`/`future-concept`/`feature`/`concept`); stripped 4 `detail_view:` references from list configs.
- Applied to `prototypes/data-entry/project/data-entry.yaml` — clean.
- `prototypes/data-entry/catalog/data-entry.yaml` deferred — has unrelated pending migrations out of scope here.
- `rela migrate --check` post-application returns "No migrations needed" — idempotent.

**E2E coverage:**
- Existing `/e2e/tests/entity-detail.spec.ts` already exercises both paths through `EntityPage.navigateToEntity` (which uses `/entity/{type}/{id}`). The bug suite covers the default-view path; the feature suite covers the configured-view path.
- Ran `npx playwright test entity-detail.spec.ts back-button.spec.ts list.spec.ts`: **29/29 passed**, including row-click navigation post-`detail_view` removal.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
