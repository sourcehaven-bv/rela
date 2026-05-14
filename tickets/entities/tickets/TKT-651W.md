---
id: TKT-651W
type: ticket
title: Remove +Add / Link Existing buttons from data-entry view widgets
kind: refactor
priority: medium
effort: s
status: done
---

## Problem

The data-entry **entity detail view** (`/entity/:type/:id`, rendered by
`EntityDetail.vue` and backed by `GET /api/v1/entities/{type}/{id}/view`) shows
relation sections (cards/list/table) and bolts inline mutation affordances onto
each section:

- `+ Add <type>` buttons that navigate to a create form pre-linked to the current entity.
- `Link Existing` button that opens `LinkExistingModal` to attach an existing entity via the section's relation.

This is conceptually wrong. The detail view is a **read** surface — the same
screen used to browse, navigate, and inspect an entity's neighbourhood. Editing
the graph from inside a read view blurs the line between viewing and editing,
and duplicates affordances that already exist in the dedicated form path
(`DynamicForm` + `SidePanel.vue`), where mutation is the explicit purpose.

The symmetric backend payload (`addInfo` / `linkInfo` on each `V1ViewSection`)
is also computed for the view path even though the form path has its own
resolver.

## Proposed change

Make entity views strictly read-only.

**Frontend (`EntityDetail.vue`):**

- Remove the `<div class="section-actions">` block that renders `+ Add` and `Link Existing` buttons.
- Remove the now-unused: `LinkExistingModal` import + render, `showLinkModal` / `linkModalInfo` state, `openLinkExisting` / `handleLinked` / `navigateToCreate` functions (verify each isn't called elsewhere first).
- Drop `addInfo` / `linkInfo` from the `ViewSection` TypeScript type in `frontend/src/api/views.ts` and `frontend/src/types/entity.ts` if they're only consumed by the view path. (`SidePanel.vue` has its own copy via `V1SidePanelSection` — check whether the type is shared or already separate.)

**Backend (`internal/dataentry/`):**

- In `api_v1.go`, drop the `a.resolveSectionButtonsWithTraverse(viewCfg, sections, result.Entry)` call in the view handler (around line 2572). Keep the call in the side-panel handler (line 1770) — the form is the legitimate place for these affordances.
- Drop `AddInfo` / `LinkInfo` fields from `V1ViewSection` (around lines 2449–2450) and the corresponding population block (lines 2675–2693). Leave `V1SidePanelSection` (lines 1687–1688) untouched.

**Keep:**

- Per-row Edit icon buttons in tables/cards. These navigate to the form for that row's entity — that's pure navigation, not a mutation, and it's the only way to reach an existing entity's edit form from inside a section.
- Top-level Edit/Delete buttons in the detail header. Those operate on the entry entity itself, not the relations.
- The whole side-panel path. SidePanel is part of the form/edit flow and legitimately offers mutations.
- The `resolveSectionButtonsWithTraverse` function itself (still used by the side-panel path).

## Acceptance criteria

1. Visiting an entity detail page shows zero `+ Add` or `Link Existing` buttons in any relation section, regardless of section display (cards / list / table / content).
2. Visiting an edit form whose `side_panel:` is configured with relation sections still shows `+ Add` / `Link Existing` buttons in the side panel — no regression in the form path.
3. `GET /api/v1/entities/{type}/{id}/view` response no longer includes `addInfo` or `linkInfo` on any section. `GET /api/v1/forms/{id}/side-panel/{entityId}` still does.
4. Per-row Edit pencil buttons in tables and cards still work and still navigate to the entity-specific edit form.
5. The header Edit/Delete buttons on the entity detail still work.
6. `just test`, `just lint`, `just arch-lint`, `just coverage-check` all pass. Frontend `npm run typecheck` clean (no orphan imports).

## Out of scope

- No changes to the form / side-panel mutation flow.
- No changes to the relation widgets in the form (`RelationCards.vue`, `RelationPicker.vue`).
- No changes to per-row Edit pencil navigation in section tables/cards.
- No changes to the document-render path (`/document/:name/:id`).

## Affected files (initial)

- `frontend/src/components/entity/EntityDetail.vue` — strip section-action block + dead imports/state.
- `frontend/src/api/views.ts` — drop `addInfo` / `linkInfo` from `ViewSection` (verify not shared with side-panel type).
- `frontend/src/types/entity.ts` — same, if applicable.
- `internal/dataentry/api_v1.go` — drop `AddInfo` / `LinkInfo` from `V1ViewSection`, drop the `resolveSectionButtonsWithTraverse` call in the view handler, drop the population block.
- Tests: any frontend/backend tests that asserted on the buttons or `addInfo` / `linkInfo` payloads.
