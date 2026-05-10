---
id: PLAN-T84HO
type: planning-checklist
title: 'Planning: Merge EntityDetail and CustomView into a single config-driven detail screen'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- Backend: re-key `_views/{viewId}/{entityId}` to `_views/{entityType}/{entityId}` (handler at `internal/dataentry/api_v1.go:2403`).
- Backend: server-side default-`ViewConfig` synthesizer for entity types without an explicit view, fed through the existing `executeView` → `buildSections` → `resolveSectionButtonsWithTraverse` pipeline.
- Frontend: merge `EntityDetail.vue` (818 LoC) and `CustomView.vue` (995 LoC) into a single renderer reached via `/entity/:type/:id`. Delete `/view/:id/:entityId` from the router.
- Frontend: drop client-side fetching of bare entity for the detail screen — always call the new view endpoint. Entity-CRUD endpoints remain in use elsewhere (forms, lists, edit).
- Migration under `internal/migration` that errors on multi-`ViewConfig`-per-type, drops `detail_view` references in list configs, and otherwise rewrites `data-entry.yaml` to the new endpoint shape.
- All in-tree `data-entry.yaml` files migrated and committed as part of this PR.
- E2E coverage for both auto-default and explicit-view paths.

Not in scope:
- `DynamicForm` and any form changes.
- Entity-CRUD endpoints (`GET/POST/PATCH/DELETE /<plural>/<id>`).
- Bespoke views (Dashboard, Analyze, Conflicts, Settings, Kanban).
- New block primitive components (`<PropertiesBlock>` etc.) — section rendering branches stay as-is post-merge.
- Backwards-compat shims for old URLs / API.

**Acceptance Criteria** (each with the test scenario that verifies it):

1. **Frontend route consolidation.** Only `/entity/:type/:id` exists; `/view/:id/:entityId` is gone.
*Test:* `frontend/src/router/index.ts` no longer has `/view/...`. E2E navigates
to old URL → SPA 404.
2. **Backend endpoint shape change.** `/_views/{viewId}/{entityId}` is gone; `/_views/{entityType}/{entityId}` serves both explicit and default views.
*Test:* Go handler test in `internal/dataentry/` for both paths and 404 for
unknown type.
3. **Server-side default-view synthesis.** When no `ViewConfig` is registered for the type, the server synthesizes one and runs it through the existing executor.
*Test:* Go test calling the new handler for a type with no config — returns
sections in the documented order.
4. **Auto-default visual parity.** Entity types without an explicit `ViewConfig` render at parity with today's `EntityDetail` for: properties order, relation sections (incoming + outgoing), content rendering, scope-nav (prev/next), back-button, `_actions` (delete affordance, transitions), edit-form navigation.
*Test:* Per-type fixture tests in Go (snapshotting the synthesized `ViewConfig`
against a metamodel fixture) plus an E2E for one representative type (e.g.
`concept` — currently EntityDetail-rendered if its config is removed).
5. **Configured-view parity.** Entity types with an existing `ViewConfig` (`idea`, `future-concept`, `feature`, `concept`) render identically to today's `CustomView`, reached through `/entity/:type/:id`.
*Test:* E2E for one representative configured type, scope-nav and edit-form
navigation included.
6. **Migration: error on multi-config.** Migration emits a clear error listing duplicates if it finds more than one `ViewConfig` targeting the same entity type, and refuses to proceed.
*Test:* `internal/migration/<new>_test.go` with a fixture YAML containing
duplicates; assert error wording.
7. **Migration: rewrite shape.** Migration removes `detail_view: {viewId}` keys from list configs (now redundant — the entity route is the only detail target) and updates `views:` to be keyed by entity type.
*Test:* Same migration test with golden before/after fixtures.
8. **In-tree configs migrated.** `tickets/data-entry.yaml`, `docs-project/data-entry.yaml`, and any other in-tree `data-entry.yaml` files are migrated and committed in the same PR.
*Test:* `git diff` review during PR; CI's `arch-lint` / app boot must succeed
against the new shape.
9. **In-tree internal navigation updated.** No `router.push({ name: 'view', ... })` or hardcoded `/view/...` strings remain in the frontend.
*Test:* Grep + lint.
10. **Cache + SSE invalidation.** The merged screen caches by `(entityType, entityId)` (matching `entitiesStore` TTL). Any entity-changed SSE event invalidates currently visible response.
*Test:* Vitest for the cache wrapper; E2E that creates an entity in another
tab/process and observes the open detail screen refreshing.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- No external library applies — this is a frontend/backend consolidation specific to rela's metamodel and view executor.
- The view executor pipeline already exists and is the right reuse point: `executeView` (`internal/dataentry/views.go:19`) → `buildSections` (`sections.go:136`) → `resolveSectionButtonsWithTraverse` (`sections.go:330`). The synthesizer just constructs a `ViewConfig` and feeds it in.
- Migration prior art: `internal/migration/dataentry_cleanup.go` and `cardinality_rename.go` — both rewrite `data-entry.yaml` in place and have golden-fixture test files. Same pattern applies.
- Auto-default ordering follows the existing `EntityDef.PropertyOrder` slice (`internal/metamodel/types.go:111`) — no new metadata needed.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical approach:**

*Backend:*
1. Adjust `handleV1Views` (`api_v1.go:2403`) to parse `/_views/{entityType}/{entityId}`. If `s.Cfg.Views` has a config for that entity type, use it. If not, synthesize a default from `s.Meta.Entities[entityType]`.
2. New synthesizer (probably `internal/dataentry/default_view.go`) builds a `dataentryconfig.ViewConfig` programmatically:
   - Section 1, `display: properties`: every property in `EntityDef.PropertyOrder` with no traverse rule (operates on entry).
   - Section N (one per outgoing relation type whose `From[]` includes this type, then one per incoming relation type whose `To[]` includes this type): traverse rule that follows the relation, display `cards` if cardinality is many-to-many or one-to-many, `list` otherwise. Heading is the relation label.
   - Section: `display: content` if the entity has body markdown.
   - Actions / transitions are not part of `ViewConfig` — they piggy-back on the entry's `_actions` / `transitions` fields, which the existing executor already attaches via `entityToV1`.
3. Index `Cfg.Views` by entity type at config-load time (or compute on demand from the `entity:` field of each ViewConfig). Cleaner: a separate `Cfg.ViewByEntity map[string]ViewConfig` populated at load.
4. Reject configs with duplicate `entity:` at load time (defensive — the migration also catches this, but config-load is the last line of defense).

*Frontend:*
1. Merge `EntityDetail.vue` and `CustomView.vue` into one component (likely keep `EntityDetail.vue` as the surviving file, since `EntityView` is already a thin wrapper — `frontend/src/views/EntityView.vue:1`).
2. Replace `entitiesStore.fetchEntity()` with a call to the new endpoint via `fetchView(entityType, entityId)` (rename or replace `fetchView` in `frontend/src/api/views.ts:105`).
3. Render sections from the response, reusing CustomView's existing `display`-branching logic (section 296–485 of CustomView).
4. Keep `useScopeNavigation` as-is — it's already shared and the data is independent of the response.
5. Delete `EntityView.vue`'s thin wrapper if the merged component is the route target directly. Delete `CustomView.vue`. Delete `/view/...` route entry.
6. Delete dead code paths: PropertyDisplay, relations rendering in EntityDetail, the entity store path used only by EntityDetail.

*Migration:*
1. New migration file under `internal/migration/`, registered via `init()` like `dataentry_cleanup.go`.
2. Reads the `views:` map. If two entries have the same `entity:`, return an error listing them with their keys. Otherwise, rewrite the map keyed by entity type.
3. Walks `lists:` and removes `detail_view:` (no longer needed).
4. Updates any `actions:` whose `script:` body contains `/view/...` URLs (none today, but checked).

*Config types:*
- `dataentryconfig.ViewConfig` already has `entity:` field — keying is already in the data, just unused in the route.

**Files to modify:**

Backend (Go):
- `internal/dataentry/api_v1.go` — rewrite `handleV1Views`, parse new path, dispatch to explicit-vs-default.
- `internal/dataentry/default_view.go` — **new file** — `buildDefaultViewConfig(meta *metamodel.Metamodel, entityType string) ViewConfig`.
- `internal/dataentry/views.go` (or wherever `Cfg.Views` is loaded) — add `ViewByEntity` index, reject duplicates at load time.
- `internal/migration/<new_name>.go` + `_test.go` — new migration.

Frontend (Vue):
- `frontend/src/router/index.ts` — remove `/view/:id/:entityId` route.
- `frontend/src/views/EntityView.vue` — keep as wrapper or merge target.
- `frontend/src/views/CustomView.vue` — **delete**.
- `frontend/src/components/entity/EntityDetail.vue` — replace with merged renderer.
- `frontend/src/components/common/PropertyDisplay.vue` — likely delete; CustomView's section rendering supersedes it.
- `frontend/src/api/views.ts` — adjust `fetchView` signature (entityType+entityId instead of viewId+entityId).
- `frontend/src/api/entities.ts` — leave entity CRUD intact, but remove any detail-screen-only consumer.
- `frontend/src/composables/useScopeNavigation.ts` — no changes expected.

In-tree configs:
- `tickets/data-entry.yaml` — re-key existing `views:` (4 entries: `idea_detail`, `future_concept_detail`, `feature_detail`, `concept_detail`) by their `entity:` value; remove `detail_view:` from corresponding lists.
- `docs-project/data-entry.yaml` (if present) — same.

**Alternatives considered:**

- **Fold view-render into entity GET** — rejected. Entity GET is used by forms, list rows, relation pickers; making it always-rich would force opt-in flags and mixed semantics on a hot path.
- **Last-wins migration for duplicates** — rejected per user input. Last-wins silently drops project intent; explicit error forces the project owner to decide.
- **Client-side default-view synthesis** — rejected during design review. The view-render shape requires server-side traversal, edit-form-ID resolution, and content embedding — duplicating that in TS is a non-starter.
- **Add a parallel new endpoint instead of re-keying** — rejected. Two endpoints with overlapping responsibilities is the problem we're removing.

**Dependencies:**
- `metamodel.Metamodel`, `EntityDef`, `RelationDef` — read-only consumers.
- `dataentryconfig.ViewConfig`, `ViewSection`, `ViewTraverse` — synthesizer constructs these; no shape changes.
- No new external Go or npm dependencies.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input sources & validation:**

- `entityType` path segment in `/_views/{entityType}/{entityId}`. Allowlist: must exist in `Meta.Entities` (existing pattern: 404 if not). No path-traversal risk because the value is keyed against an in-memory map, never used as a file path.
- `entityId` path segment. Already validated downstream by `executeView` → store lookup. Existing handler pattern preserved.
- `data-entry.yaml` content — already trusted (project file, parsed via existing config loader). Migration runs locally, no remote input.

**Security-sensitive operations:**

- File rewriting in migration: uses existing `internal/migration/yaml_util.go` helpers — same idempotency and atomic-write guarantees as existing migrations.
- No new auth, crypto, or network surfaces.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test scenarios:** mapped 1:1 to the acceptance criteria above (each AC has a
*Test:* line).

**Edge cases:**

- Entity type has no properties (empty `PropertyOrder`): properties section is omitted (or empty-state placeholder) — must match EntityDetail's behavior, which renders nothing if no properties.
- Entity type has no incoming or outgoing relations: relation sections omitted.
- Entity has no content body: content section omitted (existing `HasContent` flag path).
- Self-referential relations (entity type appears in both `From[]` and `To[]` of same relation): one section per direction, headings disambiguated by relation label vs. inverse label.
- Symmetric relations (`Symmetric: true`): one section, not duplicated.
- Multiple inverses or relation labels colliding: relations are keyed by relation name in the metamodel; collision is impossible at metamodel-load time.
- ViewConfig present but missing `entity:` field: caught at config-load time (existing validation, augmented to require `entity:` post-migration).
- Migration on a project that already has the new shape: idempotent — detects already-migrated state and is a no-op (existing migration runner pattern handles this).
- SSE event for an entity not currently visible: cache invalidation should not crash; only the cached entry for that ID is touched.
- Concurrent navigation while a fetch is in-flight: existing `usePageData` cancellation pattern handles aborts; new endpoint is a drop-in.

**Negative tests:**

- `GET /_views/unknown_type/X` → 404 with `entity_type_not_found` code.
- `GET /_views/concept/missing_id` → 404 (existing `executeView` behavior).
- Migration with two `ViewConfig` for `concept`: returns error, lists both keys, exits non-zero.
- Migration with malformed `views:` block: existing YAML-parser error path.

**Integration test approach:**

- Backend handler test that boots the dataentry server with a fixture metamodel + config and hits the endpoint for both default and explicit paths. Mirror `internal/dataentry/api_v1_test.go` patterns.
- Frontend Vitest for the merged component with a mocked view response.
- E2E (`/e2e/`) covering the navigate-to-entity-page flow for one default-view type and one explicit-view type, including scope-nav and back navigation.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|------|-----------|
| Subtle visual regressions on default-view types — e.g. property formatting, badge style, action-bar layout differs between today's hand-tuned EntityDetail and a generic section-rendered output. | Per-type fixture tests for the synthesized ViewConfig; manual smoke pass on every entity type that does not have an explicit config; release on develop branch first. |
| The synthesized default may include relation sections that EntityDetail today *doesn't* surface (e.g. internal/system relations). | The metamodel currently doesn't have a "hidden relation" concept; if differences are found, address by either filtering in the synthesizer or by adding such a concept — call out and decide during implementation, not now. |
| Cache invalidation contract change in the merged screen could mask SSE bugs that existed but were hidden by the simpler entity-store invalidation. | Vitest covering the wrapper; manual two-window test in the smoke pass. |
| Migration fails on a real-world `data-entry.yaml` with duplicates we didn't anticipate (research found none in-tree). | Error message lists offending keys; user fixes manually. Explicit by design. |
| Effort: combined backend handler + synthesizer + migration + frontend merger + in-tree config migration + E2E. | Estimated `l`. No further reduction without dropping scope. |

**Effort:** `l` (set on the ticket).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: project workflow creates docs-checklist on transition to `review`, not `in-progress`)

**Documentation impact:**

- [x] User guide / reference docs — `docs-project/`'s description of the data-entry views format needs updating (one config per type, no `detail_view` in lists).
- [x] CLI help text — no commands change. (`rela migrate` runs the new migration via existing infrastructure.)
- [x] ~~CLAUDE.md~~ (N/A: no new patterns require documenting at the project root)
- [x] README.md — only if it documents the data-entry URL scheme (it does not today).
- [x] ~~API docs~~ (N/A: no published API doc set; OpenAPI generator (FEAT-vfxz) will pick up the new shape automatically)

A docs-checklist will be created when the ticket enters `review` (per project
workflow), since this is an enhancement.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design review findings:** Conducted in conversation, applied to ticket body in
round 3. No `RR-` review-response entities were filed because findings were
addressed inline rather than tracked as separate items. Summary of what changed:
- Default-view synthesis moved server-side (was a critical contradiction with the backend solution step).
- Multi-config-per-type collapse rule made explicit (locked to **error and require human resolution**).
- Auto-default visual parity made testable via per-type fixtures.
- `_actions` / transitions / edit-form navigation added as explicit acceptance criteria.
- SSE invalidation contract added (cache by `(entityType, entityId)`, invalidate on any entity event).
- Lua / action / `detail_view` references inventoried — only `detail_view` in list configs is affected; migration handles it.
- Effort bumped from `m` to `l`.
