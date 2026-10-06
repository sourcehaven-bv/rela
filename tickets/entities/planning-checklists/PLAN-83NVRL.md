---
id: PLAN-83NVRL
type: planning-checklist
title: 'Planning: Owned items: relation targets that live only on the parent page'
started: "2026-10-05"
completed: "2026-10-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- `owning: true` on a relation type, with schema-load validation.
- Write-time rules for owning edges: one parent per child, one level deep.
- Deleting a parent deletes its owned children in the same transaction
(hard delete and soft delete/restore).
- The detail view tells the SPA the entity's readable parent; the SPA
redirects to `parent#child`.
- Search results carry the readable parent; search and the command palette
show "in <parent>" and link to `parent#child`.
- Owned children render on the parent page with scroll anchors, using
`RlRelatedList` / `RlRelatedRow` ported into
`frontend/packages/rela-components`.

Out of scope:

- More than one level of ownership.
- Owning relations to or from faced types (refused at schema load in v1).
- Folding child text into the parent's search document (decided against:
per-principal leak).
- Any default exclusion of owned items from lists, kanbans, calendars,
pickers or MCP. A list shows its own type; nothing hides them.
- Changing the 23 hand-built `/entity/...` links. The view-level redirect
covers them.
- MCP output changes (owner shown in `show_entity`); follow-up if wanted.
- Write-time enforcement of `max_incoming` in general. Only owning edges get
a write-time single-parent check.

**Acceptance Criteria:**

1. A schema with `owning: true` on a relation loads; `owning` together with
`symmetric`, or with a faced from/to type, is a load error naming the relation.
2. Creating a second owning edge into an already-owned entity is refused,
through the manager AND through automation `create_relation`.
3. Creating an owning edge whose source is itself owned, whose target
already owns something, or whose source equals its target, is refused.
4. A cascading delete (`cascade=true`) of a parent deletes its owned children, their edges, and writes an
audit record per child with `triggered_by: cascade:owner-delete:<parent>`; on
pg/sqlite a delete version is captured per child. 4a. A non-cascading delete of
a parent with children is refused with `ErrHasRelations`, as for any entity with
edges today.
5. If any child cannot be deleted by the principal, nothing is deleted and
the error does not name hidden children.
6. Soft-deleting a parent soft-deletes its children; restoring the parent
restores the children deleted with it.
7. `GET /api/v1/_views/{type}/{child}` includes `owner {type,id,title}` when
the principal can read the parent, and omits it otherwise.
8. Opening a child URL in the SPA replaces the route with
`/entity/<parentType>/<parent>#<child>` and scrolls to the child row; with an
unreadable parent the ordinary page shows.
9. A search hit on a child carries `parent` when readable; SearchView and the
command palette show "in <parent>" and navigate to `parent#child`.
10. Removing the owning edge (promotion) makes the entity a normal entity:
no `owner`, no redirect.
11. Data that breaks the rules (two owning parents, a two-level chain, a
self edge), however it got there, yields no `owner` (normal page, no redirect),
is not cascaded beyond the direct children, and is reported by a new `analyze`
check.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the model was settled in discussion with the user on 2026-10-05; decisions recorded on TKT-QO14GB)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (see above)

**Existing Solutions:**

- No library applies: this is a metamodel and write-path feature.
- Reference: Asana, Linear and Jira subtasks. A subtask is a full record
with its own assignee and status, opened in the parent's context. That is the
model chosen.
- Codebase patterns reused:
  - `entitymanager.deleteEntityInTx` / `authorizeCascadeRelations`
(`manager.go:1372,1477`) for authorizing and deleting inside one `Tx`;
`cascadeCapture` for post-commit version and audit writes.
  - `autocascade` `DeleteEntity` (`cascadehost.go:194`) as precedent for
deleting other entities from a write.
  - `linkRows` in `search_linkable.go:163`: per-page batched pairing, the
shape for the parent lookup (one `RelationQuery{EntityIDs}` per page).
  - World-absent redirect in `EntityDetail.vue:1237` as the redirect
precedent; `scrollToAnchorWhenReady` in `router/index.ts:216` already handles
`#hash`.
  - `inherit_roles_through` for operators who want children readable like
the parent; not required by this feature.
- Prior tickets: FEAT-0YL031 (inline create from relation fields), section
`create:` with `flow: modal` already creates a child from the parent page.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Metamodel. `RelationDef.Owning bool` (`types.go:1287`).
`validateRelationOwning` in `loader.go` next to the other relation validators:
refuse `symmetric`, refuse faced from/to types. Expose the flag on wire mirrors
(`apiwire RelationType`, schema JSON, MCP schema, openapi). Leave
`RenderProjection` alone; add to `ShapeProjection` only if it is a shape change
(it is not: it adds no data).
2. Write rules. In `Manager.CreateRelation` (`manager.go:2430`), for an
owning type run the create inside `Store.Tx` and check, under the lock: the
source is not the target; the target has no incoming owning edge; the source has
no incoming owning edge; the target has no outgoing owning edge. Refusals are
422 with a named rule. The check is one function, `checkOwningEdge(ctx, tx,
key)`, called from `Manager.CreateRelation` AND `cascadeHost.WriteRelation`
(automation `create_relation`, which writes the raw store). The data-entry
soft-condition fallback (`relations_modern.go:525`) writes the raw store after
the manager refused on the type allowlist, which can happen before the manager's
owning check ran; the fallback therefore calls the same check through an
exported manager method, inside `Store.Tx`, before its raw write. Importer, data
migrations and hand-edited files are operator-trusted raw writes; the read side
tolerates what they produce (step 4). The same check covers `UpdateRelation` if
it can retarget, and rename of an endpoint is unaffected (edges move with it).
3. Delete cascade, only when `cascade=true`. With `cascade=false` the
owning edges are ordinary incident edges, so the existing `ErrHasRelations`
refusal applies unchanged. In `deleteEntityInTx`, before any write: list
outgoing owning edges of the entity, authorize `OpDelete` on each child family
and its incident edges, then `DeleteFamily` each child with `cascade=true`, then
the parent. Collect each child's `cascadeCapture` so the post-commit loop writes
versions, alias notifications and audit per child. Mirror in `SoftDeleteEntity`
(the SPA path, always cascading). `RestoreEntity` needs no store marker: under
`store.WithRevealed(parent)` it lists the parent's hidden outgoing owning edges,
and restores each target that is still marked with the same `DeletedAt` as the
parent (same operation), authorizing each like the parent. No store change on
any backend. Refusal error is generic: "cannot delete: owned items you cannot
delete". It carries no child id or title. This is the same one-bit channel
`authorizeCascadeRelations` already has for hidden edges; tested for parity.
4. Owner resolution. A small `ownerOf` helper in `internal/dataentry` that,
for a page of ids, runs one `RelationQuery{EntityIDs, Direction: incoming}`
filtered to owning types and gates the parents through the request's
`visibility.Reader` in one batch. An owner is returned only when the child has
exactly one incoming owning edge, the parent is not itself owned, and parent !=
child. Anything else returns no owner. The parent's title comes from the
reader's redacted header, never the raw store row. Used by the views handler
(one id) and the search handler (page of ids).
5. Wire. `ViewResponse.Owner *EntityRef` and `LinkRow.Parent *EntityRef`
(`responses.go`), both omitted when unreadable or absent.
6. SPA. `EntityDetail.vue`: after `loadView`, if `viewData.owner` →
`router.replace({path: entityDetailHref(owner), hash: '#'+child})`, before
sections render. The views handler skips building sections when it returns an
owner, so the discarded request stays cheap. Slide panels, previews and side
panels show the child as a normal record with an "in <parent>" link; they do not
redirect (accepted). Section rows get `:id` anchors. SearchView `resultTarget()`
and CommandPalette `entityDetailHref` use `parent#child` when present and show
"in <parent title>".
7. Components. Port `RlRelatedList` / `RlRelatedRow` and their types into
`frontend/packages/rela-components`, export them, and use them for a section
with `display: related`. Row = title (link to `#child` on the same page, i.e.
selectable) plus the section's `fields` as trailing values, rendered through
`densePropertyRoutingHint` as text. No glyph and no tones in v1: enum values
carry no icon in the schema today. Glyphs are a follow-up. Existing `display:
list` gains anchors only.

**Alternatives rejected:**

- Structured list property: no relations, workflows or per-row access for
rows that carry assignees and status (user decision).
- Owned-ness on the entity type: makes promotion a type change.
- Folding child text into parent search docs: a shared index entry cannot
be filtered per principal, so it leaks a hidden child's text.
- Rewriting every link builder: 23 sites; the view-level redirect is one
choke point and fails safe (worst case one extra hop).
- Default exclusion from collection surfaces: the user wants owned items
visible, presented as part of their parent.

**Files to modify:**

- `internal/metamodel/types.go`, `loader.go` (+ tests)
- `internal/apiwire/v1/responses.go`, `internal/dataentry/api_v1.go`,
`internal/mcp/tools_schema.go`, `internal/cli/schema_json.go`,
`internal/openapi/schemas.go`
- `internal/entitymanager/manager.go`, `softdelete.go` (+ tests)
- `internal/dataentry/views_handler.go`, `api_v1.go` (search),
`search_linkable.go`, new `owner.go` (+ tests, `storetest.Counting` budget)
- `frontend/src/components/entity/EntityDetail.vue`,
`frontend/src/views/SearchView.vue`,
`frontend/src/components/ui/CommandPaletteModal.vue`,
`frontend/src/api/views.ts`, types
- `frontend/packages/rela-components/src/components/data/*` and `index.ts`
- `internal/dataentry/config.go` (`display: related`)
- `internal/entitymanager/cascadehost.go` (owning check on automation
writes)
- `internal/schema/` + `internal/cli/analyze.go` (new `owning` analyze
check)
- `docs/metamodel.md`, `docs/data-entry.md`
- e2e: `e2e/tests/owned-items.spec.ts`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `schema.yaml` `owning:` (operator config): bool; combinations validated
at load, invalid schema refuses to load.
- Relation create/delete requests (user): existing validation plus the
owning rules, enforced in entitymanager under the store lock.

**Security-Sensitive Operations:**

- Owner on view and search responses: the parent passes the request's
`visibility.Reader` before its id or title is sent. No owner field means "none
or not readable"; the two are indistinguishable.
- Cascade delete: every child is authorized with the principal's own
`OpDelete`; refusal names no child id, so it reveals at most one bit (the parent
has a child the principal cannot delete), the same one-bit membership channel
accepted for cascaded edges today.
- The redirect is client-side and uses only the owner the server already
gated; a principal who cannot read the parent gets the normal page.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1: table-driven loader tests (valid, symmetric, faced from, faced to).
- AC2-3: entitymanager tests on memstore plus the storetest backends:
second parent, owned source, target that owns.
- AC4-6: entitymanager delete tests: children gone, edges gone, audit rows
with triggered-by, version rows on sqlite (and pg when `RELA_TEST_DATABASE_URL`
is set); fs rollback-before-write check; soft delete and restore round trip.
- AC5: ACL test with a child the principal cannot delete: nothing deleted,
error text has no child id.
- AC7, AC9: dataentry handler tests with ACL fixtures: readable parent,
unreadable parent, no parent; `storetest.Counting` budget equal at 10 and 50
hits.
- AC8, AC10: Vitest for the redirect in EntityDetail; e2e spec opening a
child URL, asserting the final URL hash and the row in view; promote and reopen.
- AC9 UI: Vitest for SearchView and CommandPalette link targets and label.

**Edge Cases:**

- Parent and child the same type (recursive): list shows both; child
redirects; parent does not.
- Child with no owner after promotion: normal page.
- Parent deleted while a child page is open: SSE refetch shows not found.
- Redirect loop guard: an owner that resolves to itself is ignored (cannot
happen under the rules, guarded anyway).
- Child hash anchor not yet rendered: `scrollToAnchorWhenReady` waits up
to 2s; the parent view loads in one request.
- fs backend has no rollback: all authorization before the first write.

**Negative Tests:**

- Invalid `owning` combinations fail schema load with the relation named.
- Second parent, two-level ownership: 422, no edge written.
- Unauthorized cascade: 403/404 per existing delete semantics, nothing
deleted.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Cascade delete partially applied on fs/mem: mitigated by authorizing
every child before the first write, as `deleteEntityInTx` already does for
edges.
- Restore restores the wrong children: mitigated by deriving children from
the parent's own hidden owning edges and matching `DeletedAt`.
- Invariants broken by raw writes: mitigated by a read side that treats
any irregular shape as "no owner", plus an analyze check.
- Redirect flash (child page renders, then jumps): mitigated by redirecting
before rendering sections when `owner` is present.
- Vendored component library drift: the port copies two components and
their types; noted in the PR.

**Effort:** l

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/metamodel.md - `owning:` on relation types
- [x] docs/data-entry.md - `display: related`, navigation to owned items

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-2832P8, RR-5CJL1C, RR-65RROE, RR-GMEE4Z,
RR-LK0109 (significant, addressed in this plan); RR-8QGYU2, RR-MGV1QT,
RR-C8ZDSF, RR-ZPFWVA (minor, addressed); RR-B7AKJ8 (nit, addressed).
