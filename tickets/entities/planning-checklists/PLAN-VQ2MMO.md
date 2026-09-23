---
id: PLAN-VQ2MMO
type: planning-checklist
title: 'Planning: Query-driven entity lists in sidebar navigation groups'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Operators want a sidebar group to list live entities, for example
every active project, each entry linking to that entity.

**Decisions from the user (2026-09-23):**
- The query is an entity type plus a query scope declared in `schema.yaml` (`query_scopes:`), not a new query dialect. The user pointed at query scopes when shown the list-reference and inline-query options.
- No configurable `limit:` for now.

**Config shape:**

```yaml
# schema.yaml
project:
  query_scopes:
    active: "entity.status == 'active'"

# data-entry.yaml
navigation:
  - group: "Projects"
    items:
      - entities: project        # entity type
        query_scope: active      # optional; omitted = type's default scope; `all` = none
        sort:                    # optional; omitted = type default_sort, else id order
          - property: title
        icon: folder             # optional; applied to every link
        permission: projects:nav # optional; same UX-only filter as other entries
      - label: "All projects"
        list: all_projects
```

**Scope:**
- IN: a new `entities:` navigation entry kind with `query_scope:`, `sort:`, `icon:` and `permission:`; config validation at load; wire field on `/api/v1/_sidebar`; a SPA component that fetches the matching entities through the existing list endpoint and renders one link per entity using its display name; refresh on `entity:changed` for that type; derived postgres index for the entry's shape; docs.
- OUT: a configurable limit; free-text or search-syntax queries; `filters:`/`condition:` on the nav entry (use a scope); a new server read endpoint; per-entry counts; collapsing groups.

**Acceptance Criteria:**
1. An `entities:` entry inside a group renders one sidebar link per entity of that type matching the scope, in the configured order, labelled with the display name (falling back to the id), linking to `/entity/<type>/<id>`.
2. Only entities the principal may read appear. Membership, order and the overflow count match what the list endpoint returns for the same type, scope and sort, because the sidebar uses that endpoint.
3. Omitting `query_scope:` applies the type's `default` scope; `query_scope: all` withdraws it.
4. Invalid config fails at load with a clear message: unknown entity type, undeclared scope, unknown sort property or direction, `entities:` combined with another kind, `entities:` outside a group, or `label:` on an `entities:` entry.
5. Creating, editing or deleting an entity of that type updates the links without a page reload.
6. When more entities match than one API page holds (100, the list endpoint's `per_page` maximum), the first 100 are shown, followed by a muted "and N more" line. No new limit knob.
7. An `entities:` entry with no rows renders nothing. A group whose items all render nothing hides its heading (client-side, consistent with the server rule that drops empty groups, `views_handler.go:258`).
8. The fetch follows the current world: rows come from the active `?world=` (including `default_world`), and links carry it.
9. `/api/v1/_config` and the `/_sidebar` structure stay principal-independent apart from the existing `permission:` filter.
10. Documented in `docs/data-entry.md` and the mirrored guide.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** ~~/research~~ (N/A: the approach reuses existing list and
query-scope machinery; the user chose the config shape directly)

**Existing Solutions:**
- ~~External libraries~~ (N/A: pure config + SPA rendering over existing APIs)
- ~~Reference implementations in other projects~~ (N/A: in-codebase precedent is closer; Notion/Linear-style "favourites" sidebars are static)
- `GET /api/v1/<plural>?query_scope=&sort=&per_page=` (`internal/dataentry/api_v1.go` `scopedSortedEntitiesScoped`, `listPage`, `listpushdown.go`): ACL verdict, world, faces, query scope, store pushdown, paging, `X-Total-Count`. The sidebar consumes it as `EntityList.vue` does (it already sends `query_scope`).
- Query scopes: `internal/dataentry/queryscope.go`, config validation `checkQueryScopeRef` (`internal/dataentryconfig/validate.go:2887`).
- Derived index: `queryplan.StaticIndexSpecs` builds one spec per list via `listIndexSpec(list, meta, ev)` (`internal/queryplan/queryplan.go:199`). A nav entry has the same shape (type + scope + sort).
- Nav validation: `validateNavEntry` (`validate.go:535`). Sidebar serving: `handleV1Sidebar` / `navEntryToSidebarItem` (`views_handler.go:231,386`). SPA: `Sidebar.vue`, types in `frontend/src/types/config.ts:773`.
- SSE refresh pattern: `DocumentsPanel.vue:233` subscribes to `entity:changed`.
- History: TKT-VKM1E9 removed sidebar counts (per-request cost, staleness, aggregate leak surface). TKT-LP90MA (stale sidebar) closed as obsolete.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Config** (`internal/dataentryconfig/config.go`): add to `NavigationEntry` the fields `Entities string` (`yaml:"entities"`), `QueryScope string` (`yaml:"query_scope"`), `Sort []SortSpec` (`yaml:"sort"`). Sort uses the same `SortSpec` lists use. One helper `EffectiveNavSort(entry, meta)` returns the entry's `sort:`, else the type's `default_sort`, else nil (id order); both the wire and queryplan use it (RR: default_sort).
2. **Validation** (`validate.go`):
   - In `validateNavEntry` (no metamodel needed): `entities:` must not be combined with any other kind or with `group:`, must sit inside a group, must not carry `label:`; `query_scope:`/`sort:` are rejected on any entry that is not `entities:`. This exclusivity rule covers only `entities:`; the general case stays with TKT-VKM63H. Errors name the entity type, since the entry has no label.
   - In a new metamodel-aware nav pass: the entity type exists (checked explicitly, because `checkQueryScopeRef` returns nil for an unknown type); sort entries go through the list sort rule, extracted from `validate.go:80-84,1093-1106` into one shared helper.
   - `validateQueryScopes` gains a nav walk calling `checkQueryScopeRef`.
3. **Wire** (`internal/apiwire/v1/responses.go`): `SidebarItem` gains `Entities *SidebarEntities{Type, QueryScope, Sort string}`, where `Sort` is the effective sort already in the list endpoint's `sort=` grammar (`title,-due`). The SPA derives the URL segment with the existing `getPlural()`. `navEntryToSidebarItem` fills it and sets `Href` empty. `handleV1Sidebar` is otherwise unchanged; `permission:` filtering applies as for every entry.
4. **SPA**:
   - `frontend/src/types/config.ts`: `SidebarEntities` type, optional `entities` on `SidebarItem`.
   - New `frontend/src/components/common/SidebarEntityLinks.vue`: a Pinia Colada `useQuery` keyed `entityKeys.listParams(type, params)` over the existing list API client, params = `query_scope`, `sort`, `per_page=100` and the current `worldParam`. `useEvents` already invalidates `entityKeys.type(type)` on `entity:changed` and the root key on `refresh`, and the server batches events per type every 200 ms, so there is no manual subscription or debounce. The overflow line reads `ListResponse.meta.total`. Links are `RouterLink`s keyed and addressed with `entityRef` (face-aware), carry `query.world`, use `NavIcon`, and show the display name (id fallback) with `title`/`aria-label` for the collapsed sidebar. A fetch error renders a muted "Could not load" line and logs; it never looks like an empty result.
   - `Sidebar.vue`: render `SidebarEntityLinks` for items with `entities`; key them on type + scope + index (the current label+href key collides); hide a group heading whose items all render nothing; reload `/_sidebar` on the `refresh` SSE event so a hot config reload does not leave a stale entry requesting a removed scope.
5. **Derived index** (`internal/queryplan/queryplan.go`): in `StaticIndexSpecs`, feed each nav `entities:` entry through `listIndexSpec` as a synthetic `dataentryconfig.List{EntityType, QueryScope, Sort: EffectiveNavSort(...)}`. Initialise the lazy evaluator for nav entries too, and extract the dedupe key into a helper shared with lists. Scoped entries are not store-paged (see Risks), so the index serves the scope prefilter, not ORDER BY + LIMIT; `queryplan_test.go` pins that a nav entry yields the same spec as an equivalent list.
6. **Docs**: `docs/data-entry.md` Navigation section (new "Entity lists" subsection, direct-items table row; entries are never landing targets; the overflow line has no destination, add a `list:` entry for that; links go stale after a grant change until the next write or reload) and `docs-project/entities/guides/GUIDE-data-entry.md`. `docs/metamodel.md`: `default` scope also applies to nav `entities:` entries. `docs/acl-security.md` + `GUIDE-acl-security.md` "Sidebar menu structure is principal-independent": state that `entities:` entries carry only the query definition, and the SPA fetches rows through the ACL-scoped list endpoint, so the sidebar response still carries no per-principal data. Fix the stale "only *counts* are gated" wording in root `CLAUDE.md`.

**Alternatives considered:**
- *Server expands entities into the `/_sidebar` response.* Rejected: puts per-principal entity rows into the sidebar payload, adds a second gated read path (the leak surface TKT-VKM1E9 removed), and makes every sidebar load pay for every query.
- *New dedicated endpoint `/_sidebar/entities/{n}`.* Rejected: duplicates the list endpoint's ACL/world/face/scope/pushdown handling, which `scopedread.go` warns must not be reimplemented per handler.
- *Reference a `lists:` entry, or inline search syntax.* Offered; the user chose type + query scope.
- *Configurable `limit:`.* Declined by the user for now; the list endpoint's existing 100-row page is the only bound.

**Dependencies:** existing list API client in `frontend/src/api`, `useEvents`
composable, display-name helper, `NavIcon`; Go: `dataentryconfig`, `apiwire/v1`,
`queryplan`.

**Files to modify:**
- `internal/dataentryconfig/config.go`, `validate.go`, `validate_test.go`
- `internal/apiwire/v1/responses.go`
- `internal/dataentry/views_handler.go` (+ new `sidebar_entities_test.go`)
- `internal/queryplan/queryplan.go`, `queryplan_test.go`
- `frontend/src/types/config.ts`, `frontend/src/components/common/Sidebar.vue`, new `SidebarEntityLinks.vue` + `SidebarEntityLinks.test.ts`
- `e2e/tests/sidebar-entities.spec.ts` (new)
- `docs/data-entry.md`, `docs/acl-security.md`, `docs-project/entities/guides/GUIDE-data-entry.md`, `GUIDE-acl-security.md`, `CLAUDE.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `data-entry.yaml` nav entry (operator config, trusted, public per CLAUDE.md): validated at load against the metamodel (type, scope name, sort properties, allowlisted directions). Invalid config refuses startup/reload.
- Request parameters to the list endpoint come from the served config, but a client can send anything; the endpoint validates `query_scope` (400 on unknown/duplicate). `sort=` accepts any property name (harmless: unknown properties sort as absent). Nothing new is accepted server-side.

**Security-Sensitive Operations:**
- Entity reads: all through the existing list endpoint, so row-level ACL, world, face allowlist, field redaction and the uniform empty result for DenyAll apply unchanged. A redacted display property falls back to the id, as everywhere else.
- `X-Total-Count` is the post-gate scoped count the list endpoint already serves to the same principal; the overflow line adds no new existence channel.
- SSE `entity:changed` is already ACL-filtered per connection (TKT-POT9GQ); a refetch triggered by it runs through the gate again.
- `query_scope` is not access control (docs say so); `permission:` stays a UX filter. Docs will repeat both points for this entry kind.
- No new per-principal data on `/_config` or `/_sidebar`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1: Vitest `SidebarEntityLinks.test.ts` renders links with display names, id fallback, correct hrefs and order from a mocked list response; e2e spec seeds three projects (two active) and asserts two links in sort order, clicking one opens its detail page.
- AC2: Go test in `sidebar_entities_test.go` asserts the `/_sidebar` item carries the type/plural/scope/sort that, fed to the list endpoint as a principal with a partial read grant, returns only readable rows (reuses the list-endpoint ACL fixtures). e2e with an ACL project: a restricted user sees only permitted projects.
- AC3: Go test that the served `query_scope` is empty when omitted (list endpoint applies default) and `all` when set; existing list tests cover the endpoint semantics.
- AC4: table-driven `validate_test.go` cases for every rejection listed.
- AC5: Vitest: invalidating `entityKeys.type('project')` refetches the entry; another type does not. e2e: create an active project via the API, link appears without reload.
- AC10: Vitest: the query params and key include the world; links carry `query.world`. e2e with a two-world project.
- AC6: Vitest: `X-Total-Count: 130` with 100 rows renders "and 30 more".
- AC7: Vitest: empty response renders nothing; a group whose items all render nothing has no heading.
- AC8: extend `TestNavPermission_ConfigUnfiltered` style test: `/_config` identical across principals; `/_sidebar` differs only by `permission:`.
- `queryplan_test.go`: nav entry produces the same `DerivedObjectSpec` as an equivalent list; duplicates collapse.

**Edge Cases:**
- Type with no declared scopes and no `query_scope:` → unscoped read.
- `query_scope: all` on a type with a default → default withdrawn.
- Scope referencing `current_user` → list endpoint binds identity (existing behaviour); covered in the Go test and in e2e with two users.
- Display property empty or redacted → id shown.
- Entity type unreadable for principal (DenyAll) → empty, identical to a type with no matches.
- Two `entities:` entries for the same type → both refresh on one event, each debounced independently.
- Rapid bursts of `entity:changed` → one refetch per debounce window.
- Sidebar collapsed → icon-only links.
- Hot config reload changes the entry → sidebar reloads it on next mount (same as today's sidebar); documented.

**Negative Tests:**
- Config: unknown type, undeclared scope, unknown sort property, bad direction, `entities:` + `list:`, `entities:` at top level, `label:` on `entities:`, `query_scope:` on a `list:` nav entry → load error naming the entry.
- SPA: list fetch 500 → "Could not load" line, not an empty list.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- *Refetch cost on busy deployments* (the concern behind TKT-LP90MA/TKT-VKM1E9): a scoped entry is NOT store-paged (`api_v1.go:393`); each refetch reads every row the scope's pushable conjuncts match, re-checks the scope and sorts in Go, and the handler also loads page edges. That is what the equivalent scoped list page costs today, paid once per client per write-batch to that type. Mitigations: server batches events per type (200 ms), the query cache deduplicates, the scope's store-safe conjuncts narrow the read. Measure a scoped entry on the postgres perf seed with `rela-server -verbose` during implementation; if it is poor, stop and bring it to the user (options: store-paged scoped reads, or a limit).
- *Stale after grant change*: no `entity:changed` on a role-conferring write; documented.
- *Sidebar grows long* with no limit: bounded by the 100-row page; the user accepted no knob for now.
- *Config shape drift between list and nav sort grammar*: reuse `SortSpec` and one serializer to the `sort=` grammar.
- *Index derivation disagreeing with runtime*: reuse `listIndexSpec` rather than a copy.

**Effort:** m

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - new `entities:` navigation entry
- [x] docs/acl-security.md - sidebar section clarified
- [x] CLAUDE.md - stale "only counts are gated" wording
- [x] ~~docs/metamodel.md~~ (N/A: query scopes unchanged)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] Mirrors in docs-project/entities/guides/ (GUIDE-data-entry, GUIDE-acl-security)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 19 findings (5 significant, 10 minor, 4 nit), all
addressed in this plan and recorded as review-responses linked via
has-review-response.
