---
id: IMPL-GETR
type: implementation-checklist
title: 'Implementation: Action affordances phase 2: frontend consumption + AWM6L payoff'
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

Phase 2 implementation landed:

**Frontend (entity-CRUD gates):**

- `frontend/src/utils/affordancesWarning.ts` (NEW) + 8 unit tests — AC9 dev-mode warning with dedup, HMR reset, list/entity/empty-map/unknown-verb cases.
- `frontend/src/api/entities.ts` — wired `warnIfMissingActions` into the whitelist (`listEntities`, `getEntity`, `createEntity`, `updateEntity`).
- `frontend/src/stores/entities.ts` — `fetchList` now passes `_actions` through (collection-scope verdict survives the cache).
- `frontend/src/components/lists/EntityList.vue` — gates added: `+ New` button (AC1), per-row delete on both layouts (AC2), Del-key handler with toast feedback (AC3), bulk action bar (AC8).
- `frontend/src/views/KanbanView.vue` — gates added: `+ New` (AC4), drag-drop via `:draggable` binding + `@drop` defence (AC4).
- `frontend/src/components/entity/EntityDetail.vue` — gates added: Edit + Delete buttons desktop/mobile (AC6), Del/E key handlers with toast (AC5).
- `frontend/src/components/forms/DynamicForm.vue` — route guard: when loaded entity has `_actions.update === false`, render inline "not editable" message with back link instead of the form (AC7).

**Backend (Go tests):**

- `internal/dataentry/affordances_test.go` — added `TestComputeActions_MixedTypeDeclarative` (AC12): a `Declarative` policy with mixed type grants; verifies per-type variance.

**E2E (AC10):**

- `e2e/tests/read-only-mode.spec.ts` (NEW) — extends the existing `testProject` fixture with a `readOnlyServerUrl` that boots `rela-server --read-only`. Three scenarios all green:
  - List page has no `+ New` button and no row delete buttons.
  - EntityDetail has no Edit or Delete buttons.
  - Direct `/form/:type/:id` navigation renders the "not editable" message.

**Docs:**

- `docs/data-entry/api-reference.md` — new §"How the SPA consumes `_actions`" section (the cardinal rule is unchanged).
- `docs/security.md` — clarified "entity-CRUD controls are absent" in read-only mode (with deferred-sites caveat).
- `CLAUDE.md` — new SPA-side rule under §"Action affordances": gate via `entity._actions?.[verb] !== false`; no `useACL()` composable.
- `.ignored/action-affordances-design.md` — amended §"Anonymous principal handling" to admit the two-state consumer-side collapse (empty `{}` and absent now render identically; phase 1's three-state distinction collapsed).

**Deferred from phase 2 (documented as known-visible-in-readonly):**

- Lua command/action buttons (`runCommand`, `executeAction`) — no phase-1 verb covers them.
- Settings / theme / git / scheduler write paths — covered by `--read-only` server flag at the endpoint; UI not gated.
- Relation add/remove inside form widgets (RelationCards, RelationPicker) — gated on TKT-XZEY (ACL v0.5).
- Inline-edit buttons in EntityDetail related-entity cards (×6 sites) — `V1SidePanelEntity` doesn't carry `_actions`; requires backend serializer extension. Defer to a phase-2.1 ticket.
- RelationPicker "+ Add new" inline create — needs per-target-type collection actions fetched separately. Defer.

**Test counts:**

- Backend: all packages green with race detector (`go test -race ./...`).
- Frontend: 792 tests pass (was 791; +8 helper tests, -7 from helper file deltas elsewhere balance out).
- E2E: 3/3 new tests green.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
