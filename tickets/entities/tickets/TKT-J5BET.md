---
id: TKT-J5BET
type: ticket
title: Merge EntityDetail and CustomView into a single config-driven detail screen
kind: enhancement
priority: medium
effort: l
status: in-progress
---

## Problem

The data-entry UI has two screens that render the same conceptual thing — the
detail page for a single entity — through two completely separate code paths:

- `/entity/:type/:id` → `EntityDetail.vue` (818 lines, hardcoded layout) backed by the generic entity REST endpoint (`GET /<plural>/<id>`), which returns a flat `{id, type, properties, content, relations}` shape with relations as ID lists only.
- `/view/:id/:entityId` → `CustomView.vue` (995 lines, config-driven via `ViewConfig.sections`) backed by `GET /api/v1/_views/{viewId}/{entityId}` (handler in `internal/dataentry/api_v1.go`), which executes the view's traverse rules server-side and returns rich pre-resolved sections (cards, rows, columns, groups, embedded content, edit-form IDs).

Both surfaces reimplement: page header, back/scope navigation, property
listings, relation listings (button-list / cards / list / table branches),
content rendering, and action bars. The split forces every per-type detail
customization to be done either by editing the hardcoded `EntityDetail` or by
writing a full `ViewConfig` — there is no middle ground.

In practice nobody uses more than one view per entity type, so the conceptual
split between "default detail" and "custom view" buys no flexibility — only
duplication. The duplication exists on the backend too: a generic entity
endpoint *and* a view-rendering endpoint, both serving the same kind of read.

## Solution outline

Collapse to a single detail screen, always config-driven. Clean break — no
backwards-compat layer:

1. **Backend.** Re-key the existing handler at `internal/dataentry/api_v1.go:2403` from `/_views/{viewId}/{entityId}` to `/_views/{entityType}/{entityId}`. The endpoint always returns the rich pre-resolved shape that `CustomView` consumes today. When no `ViewConfig` is registered for the requested entity type, the **server synthesizes a default `ViewConfig` from the metamodel** and executes it through the same pipeline. Default-view synthesis lives on the server, not the client — the existing executor (`executeView`, `buildSections`, `resolveSectionButtonsWithTraverse`) is the only place that produces the right shape. Configs with duplicate `entity:` are rejected at config-load time as a defensive gate (the migration also catches this).

2. **Auto-default specification.** The synthesized default produces, in this order: (a) one `properties` section listing every visible property in `EntityDef.PropertyOrder`; (b) one section per outgoing relation type whose `From[]` includes the entity type, then one per incoming relation whose `To[]` includes it, with `cards` or `list` display chosen by cardinality; (c) one `content` section if the entity has body markdown. Actions and transitions ride on the entry's `_actions`/`transitions` fields — already attached by the existing executor via `entityToV1`. Per-type fixture tests in Go snapshot the synthesized output against a metamodel fixture.

3. **Frontend.** Keep `/entity/:type/:id` as the canonical route. Delete `/view/:id/:entityId`. The merged screen always calls the new endpoint (no client-side view synthesis). Update all internal navigation to the entity route. Delete the obsolete code in `EntityDetail.vue` / `CustomView.vue`. Old `/view/...` URLs hit the SPA's existing 404 — no redirect or shim.

4. **Migration.** Add a migration under `internal/migration` that rewrites existing `data-entry.yaml` files to the new shape: re-key `views:` by entity type, drop `detail_view:` from list configs (now redundant since the entity route is the only detail target), and drop entries equivalent to the auto-default. **If multiple `ViewConfig` entries target the same entity type, the migration errors with a clear message listing all duplicates and refuses to proceed** — the project owner must resolve manually. Migration follows the existing pattern (`dataentry_cleanup.go`, `cardinality_rename.go`). Research found no in-tree duplicates today.

5. **SSE and caching.** The merged screen caches by `(entityType, entityId)` matching today's `entitiesStore` TTL. Any entity-changed SSE event invalidates the cached response. Existing `useEvents` invalidation flow extends to the new cache.

## Non-goals

- No changes to `DynamicForm`.
- No changes to bespoke views (Dashboard, Analyze, Conflicts, Settings, Kanban).
- No changes to entity CRUD endpoints used by forms/list/edit. Only the read-side view-render endpoint changes.
- No backwards-compat for old `/view/:id/:entityId` URLs or the old `_views/{viewId}` API. External bookmarks and external clients are not preserved; old URLs hit 404.
- No new block primitive components on the frontend. Section rendering branches stay as they are post-merge — extracting `<PropertiesBlock>` etc. is a follow-up.

## Acceptance criteria

- One frontend renderer, one route. Only `/entity/:type/:id` exists; `/view/:id/:entityId` is gone from the router.
- Backend: `/_views/{viewId}/{entityId}` is gone, replaced by `/_views/{entityType}/{entityId}`.
- Server-side default-view synthesis: when no `ViewConfig` is registered for the requested entity type, the new endpoint synthesizes one and executes it through the same pipeline.
- Entity types without an explicit `ViewConfig` render at parity with today's `EntityDetail`. Verified by per-type fixture tests covering: properties order, relation sections (incoming + outgoing), content rendering, scope-nav (prev/next), back-button, `_actions` (delete affordance, transitions), and edit-form navigation.
- Entity types with an existing `ViewConfig` render at parity with today's `CustomView`, reached through `/entity/:type/:id`. Same affordances verified.
- Migration under `internal/migration` upgrades existing `data-entry.yaml` files to the new shape on next run, with a conformance test using example fixtures. **Errors and refuses to proceed** when more than one `ViewConfig` targets the same entity type.
- Migration removes `detail_view:` from list configs.
- All in-tree `data-entry.yaml` files (project configs under `tickets/`, `docs-project/`, etc.) are migrated and committed as part of the same change.
- All in-tree references to the old view ID — including Lua scripts and action bodies — are inventoried, migrated, or documented as breaking.
- All in-tree internal navigation uses the entity route — no leftover `/view/...` links.
- Cache + SSE invalidation works on the merged screen, keyed by `(entityType, entityId)`.
- E2E coverage exists for both default-view and configured-view paths.
