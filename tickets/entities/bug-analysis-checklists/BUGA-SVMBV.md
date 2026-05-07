---
id: BUGA-SVMBV
type: bug-analysis-checklist
title: 'Analysis: Detail-view list section items are not clickable (no href, broken router push)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally

Defined a custom detail view in `data-entry.yaml` with a section using `display:
list` that traverses an outgoing relation. Opened the detail view in the
browser, scrolled to the section, clicked an item.

```yaml
views:
  detail_policy:
    title: 'Beleid'
    entry: { type: beleid }
    traverse:
      - from: entry
        follow: schrijft_voor
        collect_as: out_procedures
    sections:
      - heading: 'Schrijft procedures voor'
        source: out_procedures
        display: list
        fields:
          - property: titel
          - property: status
```

- [x] Minimal reproduction steps documented

  1. Configure a `view` with at least one `display: list` section that
traverses to a related entity type.
  2. Open the view: `/view/detail_policy/POLICY-001`.
  3. Click any item in the list section. Expected: navigate to that
entity's detail page. Actual: nothing happens.

- [x] Environment/conditions noted

Reported on macOS arm64, `rela` dev binary 30 apr 2026 (commit 0bbec24).
Reproduces in any browser.

## Root Cause

- [x] Immediate cause identified (why1)

`navigateToEntity` at `frontend/src/views/CustomView.vue:126-128` calls
`router.push({ name: 'entity', params: { id: entityId } })` but the named route
`entity` is `path: '/entity/:type/:id'`. Vue-router rejects with
`MISSING_REQUIRED_PARAMS` and `router.onError` silently swallows it (no console
error, no nav).

- [x] Contributing factors found (why2-3)

  - Why 2: `navigateToEntity(entityId)` accepts only an id, even though the
API response includes the type for every list/card/content item.
  - Why 3: There is no test exercising the click path on a custom view's
list/cards/content section.

- [x] Systemic cause explored (why4-5)

  - Why 4: "Where do you view an entity of type X" is currently encoded on
*lists* (`lists.<id>.detail_view`) — but a list isn't the right anchor for that
question. `EntityList.vue` happens to be inside a list when navigating, so it
has unambiguous context. `CustomView.vue` doesn't, and there's no clean "the
default detail view for type X" lookup.
  - Why 5: Entity-type-level UX defaults aren't representable. Adding
`entity_views.<type>.detail_view` to `data-entry.yaml` fixes both this bug and
the broader question once.

## Fix Planning

- [x] Fix approach determined

### Config schema change (Phase 1 of two-phase plan)

Add a new top-level `entity_views:` section to `data-entry.yaml`:

```yaml
entity_views:
  procedure:
    detail_view: detail_procedure
  beleid:
    detail_view: detail_beleid
```

**Key name** is `entity_views`, not `entity_types`, to avoid collision with
existing `commands.available_on.entity_types` and user-defaults
`overrides.entity_types` (both `[]string` lists). Verified existing usages
during second design review.

Semantics: `entity_views.<type>.detail_view` is *the* canonical detail view for
entities of that type. Used by both `EntityList.vue` (replacing
`lists.<id>.detail_view` after migration) and `CustomView.vue` sections.

Future Phase 2 (out of scope): per-section
`views.<viewId>.sections[].link_to_view` override. Lookup chain becomes
`section.link_to_view → entity_views.<type>.detail_view → /entity/:type/:id`.

### Migration

New `internal/migration/detail_view_to_entity_views.go`. Apply:

1. Group lists by `entity_type`. Collect distinct `detail_view` values per group.
2. Migrate-able group (single distinct value, or all match): write
`entity_views.<entity_type>.detail_view: <view>` (merge into existing
`entity_views:` if present), delete `detail_view:` from all lists in the group.
3. Conflicting values: skip the group entirely. The migration neither writes
to `entity_views` nor strips from lists. The conflict is a config error to be
surfaced via `rela validate` separately (out of scope: `validate.go` can warn
that `lists.<id1>.detail_view` and `lists.<id2>.detail_view` differ for the same
entity type — that's a separate enhancement).
4. **`Detect()` must be idempotent**: it returns `true` only when `Apply()`
will produce a non-empty change. Specifically: at least one migrate-able group
exists. After migration of all migrate-able groups, only conflicting groups
remain — `Detect()` returns `false` and the server starts. The conflict is
permanent until the user reconciles it manually.

**YAML insertion**: The new `entity_views:` block is inserted *after* `lists:`
(or after `forms:` if `lists:` is absent) using a new `yaml_util.go` helper
`InsertMapKeyAfter(root, anchor, key, valueNode)`. Rationale: `entity_views` is
conceptually adjacent to `lists`/`views` and reads naturally there.

**Migration pre-existing `entity_views:` keys**: if the user has hand-written or
partially migrated a file, merge into the existing map instead of overwriting.
Conflict between hand-written and migrated values: skip that type (treat as
"user knows best"), leave list-level intact, surface in validate.

### Backend (Go)

- `internal/dataentryconfig/config.go`: add
`EntityViews map[string]EntityViewConfig` to `Config`, with
`EntityViewConfig{DetailView string \`yaml:"detail_view"
json:"detail_view,omitempty"\`}`. JSON-tagged so it ships in the config
response. Kept distinct from frontend's `EntityType` (metamodel) by name
(`EntityViewConfig`).
- `internal/dataentryconfig/validate.go`:
  - Add `entity_views` to `validTopLevelKeys` map at line 16.
  - Validate referenced view exists (mirror existing list `detail_view`
check at line 1087).
- **No SPA-side back-compat fallback.** The migration is mandatory —
`internal/dataentry/app.go` already bails on detected migrations and instructs
the user to run `rela migrate`. After migration, list-level `detail_view` is
gone; the SPA only uses `entity_views`. `lists.<id>.detail_view` remains
parseable in the Go config struct (don't break existing `data-entry.yaml` files
mid-edit), but is unused by both backend and frontend post-migration.

### Frontend (Vue)

- `frontend/src/types/`: add `EntityViewConfig` interface with `detail_view?: string`.
Distinct name from the existing `EntityType` (metamodel) to avoid import
confusion.
- `frontend/src/stores/schema.ts`:
  - Load `entity_views` from `getConfig()` response into a new
`Map<string, EntityViewConfig>` ref called `entityViewConfigs` (distinct from
existing `entityTypes` for the metamodel).
  - Add `getEntityDetailView(type)` returning the configured view id or
undefined.
- `frontend/src/utils/entityRoute.ts` (NEW): export pure helper
`entityDetailHref(entity: { id: string; type: string }, getDetailView: (type:
string) => string | undefined, opts?: { cellLink?: string }): string`. Helper
takes a *callback* for detail-view lookup, not the schemaStore — keeps it
testable without Pinia. Returns:
  - `cellLink` if provided (table cells have server-resolved per-column
links — preserve them)
  - `/view/${detailView}/${entity.id}` if the entity-type has
`detail_view`
  - `/entity/${entity.type}/${entity.id}` as the floor (always
non-empty when `entity.type` is non-empty)
  - **Empty `entity.type` fallback**: returns empty string. Templates
guard with `v-if="href"` and skip rendering an anchor in that case. Unit test
asserts both the contract and the rendering path.
- `frontend/src/views/CustomView.vue`:
  - List display: `<a class="list-link" :href="hrefFor(entity)"
@click.prevent="navigateToEntity(entity)">` with `v-if="hrefFor(entity)"` guard
for empty-type case. Add CSS to `.list-link`: `text-decoration: none; color:
inherit;` and a `:focus-visible { outline: 2px solid var(--accent-color);
outline-offset: 2px; }` rule for keyboard a11y.
  - Cards / content cards: keep `<article>`, change handlers to pass
`entity` (full object) instead of `entity.id`. (Out of scope to convert to
anchors — separate ticket would do that for full a11y.)
  - Table cells: keep server `cell.link` as `:href` (preserve per-column
link behavior). Click handler builds entity from `cell.entityId/entityType`
falling back to `row.entityId/entityType`. The `entityType` field is already on
rows but **not on cells** today; backend addition needed:
`SectionColumnData.EntityType` is already populated to row's type at
`sections.go:217`, so it's `row.entityType` (table-row level), not a new field.
Click handler uses `{ id: cell.entityId || row.entityId, type: row.entityType
}`. Pre-existing relation-cell-points-at-row issue accepted as out of scope —
code comment flags it.
  - `navigateToEntity(entity)` builds path via the helper. **Back-nav uses
`return_to=/view/<viewId>/<entityId>` query param**, NOT `from=<viewId>` (which
would route to `/list/<viewId>` and 404). `useBackTarget` already supports
`return_to` as the higher-priority path. Trade-off: no scope-nav (prev/next
inside the view) — acceptable for v1, views don't have a clear next-entity
semantic.
- `frontend/src/components/lists/EntityList.vue`: replace
`listConfig.value.detail_view` lookup with
`schemaStore.getEntityDetailView(entity.type)`. Migrate to use the new
`entityDetailHref` helper to keep one source of truth for the priority chain.
Existing column-link still wins (passed via `opts.cellLink`).

### In-tree config migration (PR-included)

**This PR migrates and commits**:
- `tickets/data-entry.yaml` (idea, future-concept, feature, concept all have
single `detail_view` lists; no conflicts expected).
- `prototypes/data-entry/project/data-entry.yaml` (ticket, category).
- `prototypes/data-entry/catalog/data-entry.yaml` (verify if any
detail_view present).

**CI guard**: a Go test in `internal/migration/` walks repo for
`data-entry.yaml` files and asserts `Detect()` returns false against each.
Catches "someone added a new detail_view in a list and didn't re-migrate."

**Migration impact on sub-lists**: `tickets/data-entry.yaml` has e.g. `idea`
with three lists (`all_ideas` has detail_view, `active_ideas` and
`game_changers` don't). After migration, all three inherit `idea_detail`.
**Decision**: accept this as expected behavior — subset/filter lists should send
users to the same detail page as the canonical list. Document in the migration's
package-doc comment.

### Explicitly out of scope

- Per-section `link_to_view` override (Phase 2). Plan composes cleanly
when added later.
- Fixing relation-column-cell navigation. Pre-existing issue.
- Removing list-level `detail_view` from the Go config schema entirely
(parsed but unused; safe to remove in a follow-up cleanup ticket once the
migration has had time to land everywhere).
- `display: properties` on non-entry source dead-code path. Latent issue.
- Migrating `SidePanel.vue` / `RelationCards.vue` to the helper
(already correct, not affected by the bug).
- Wrapping cards/content-cards in real anchors for full a11y
(separate enhancement ticket).
- `rela validate` warning on inter-list `detail_view` conflicts
(separate enhancement; today the migration just leaves them and the config keeps
working with list-level fallback gone).
- Server-side path resolution (emitting `entity.detailHref` in API
response). TODO comment in helper file flags as future direction.

- [x] Regression test planned

**Backend Go tests**:
- Migration: single-list, multi-list-same-detail, multi-list-conflict
(no-op: Detect false), no-detail (no-op), pre-existing partial `entity_views:`
(merge), idempotency (run twice, second pass = no-op).
- Config JSON serialization for `entity_views`.
- Validate rejects unknown view ref in `entity_views.<type>.detail_view`.
- Validate accepts missing `entity_views:` (universal pre-migration case).
- In-tree configs Detect=false guard test.

**Frontend Vitest tests**:
- Unit test for `entityDetailHref` helper covering all branches:
cellLink provided, type with detail_view via callback, type without detail_view
(falls back to `/entity/...`), empty type (returns empty string, not malformed
`/entity//id`), all permutations.
- Component test for `CustomView.vue`: mount with stubbed `fetchView`
returning a `display: list` section, assert (a) the rendered `<a>` has the
expected `href`, (b) clicking dispatches `router.push` with the correct path +
`return_to` query.
- Component test for `EntityList.vue` after migration: assert the new
helper is used, column-link still wins.

**E2E test** (Playwright in `e2e/tests/custom-view-list-navigation.spec.ts`):
open a project fixture with a custom view containing a `display: list` section,
click an item, assert URL changes to either the configured detail_view or
`/entity/:type/:id`. Verify back-button (browser back + the BackButton
component) returns to the originating custom view.

**Manual a11y check during implementation**: tab to `.list-link`, see focus ring
(CSS `:focus-visible`), hit Enter to navigate. Verify with screen reader that
`<a href>` is announced as "link" and the entity title is read.

- [x] Related areas checked for similar issues

`grep "name: 'entity'"` and `grep "/entity/"` across `frontend/src`:
- `CustomView.vue` (broken, this fix)
- `EntityList.vue` (correct, will be migrated to use new helper)
- `SidePanel.vue`, `RelationCards.vue` (correct, use path strings, will
*not* be migrated in this fix to keep diff focused)
