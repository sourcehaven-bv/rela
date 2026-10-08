---
id: PLAN-MR2QHV
type: planning-checklist
title: 'Planning: Drag-and-drop reorder of a list scoped to one entity through an orderable relation'
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Orderable relations exist since TKT-XF5F: `orderable: outgoing|incoming|both` on
a relation type, managed `_order_out` / `_order_in` edge values, append on
create, renumber on collapse. Only the edit form's relation cards use them. The
user wants the order shown and draggable wherever a collection is "the children
of one anchor over one relation". Concrete Atlas examples, all over
`bestaat_uit` (topic → taak, outgoing):

1. entity-page list tab `tabel` (`list:` + `scope: {relation: bestaat_uit}`);
2. entity-page kanban tab `board` (vertical order within a column);
3. detail-view section `Onderdelen` (`traverse: follow: bestaat_uit`,
`display: table`).

**Scope:**

In scope:
- One server concept, "relation-ordered collection": a collection whose rows
are the targets of ONE anchor's outgoing edges over ONE relation whose outgoing
side is orderable (`_order_out`).
- Automatic: such a collection is shown in relation order unless the reader
picks a column sort (lists) or the section declares `sort:` (sections). Relation
order replaces the list's `default_sort` on a scoped tab.
- Drag-to-reorder plus a keyboard path (Alt+↑ / Alt+↓) on all three surfaces.
- A server-side position write on the existing relation PATCH (before/after a sibling, or step) so paging, filtering
and stale client state cannot produce a wrong value.
- Kanban: drop at a position within a column (reorder) and into another
column at a position (status change, then move).
- Docs: `orderable` in docs/metamodel.md (currently undocumented) and the
reorder behaviour in docs/data-entry.md.

Out of scope:
- Incoming-side ordering (`_order_in`, anchor is the target): faced sources and per-row permissions make it a separate design; follow-up ticket.
- Recursive / nested sections (`parent_sort`/`child_sort`), multi-hop
traversals, and collections fed by more than one source entity: no single
anchor, so no single order.
- Global entity ordering independent of a relation.
- Sidebar `entities:` nav lists and dashboards.
- Touch drag (the board library has none; keyboard path covers a11y).
- Atlas config: the operator adds `orderable: outgoing` to `bestaat_uit`.

**Acceptance Criteria:**
1. On a list tab scoped by an orderable relation and no reader sort, rows come
back in relation order; edges without a value sort last, ties by id. Test: API
test with three tasks ordered 3,1,2 returns them in that order.
2. Dragging a row by its handle (or Alt+↑/↓ on the handle) moves it; after reload the new order
persists. Test: e2e on a project page tab.
3. Clicking a column header sorts by that column and hides the drag handles;
clearing the sort restores relation order and handles. Test: component test.
4. A grouped list shows no drag handles. Test: component test.
5. Moving across a page boundary works (before the first row of page 2 lands
between page 1's last row and it). Test: entitymanager + API test.
6. Kanban tab: cards in a column follow relation order; dropping between two
cards in the same column reorders; dropping into another column at a position
sets the column property and the position. Test: component test and e2e.
7. Detail section fed by exactly one flat `from: entry` rule over an outgoing-orderable relation, without `sort:` or `group_by`, shows
relation order and is draggable; with `sort:` it is not. Test: Go section test
and component test.
8. A principal without update permission on the relation, or with `_order_out`
not writable, gets `movable: false` (no handles) and the PATCH answers 403.
Test: API test.
9. A move naming a sibling the principal cannot read answers the same 404 as a
nonexistent sibling. Test: API test.
10. The prev/next navigator on a relation-ordered tab walks the same order as the tab. Test: useScopeNavigation unit test.
11. Starting from edges with no order values, the first move puts the row exactly where it was dropped. Test: entitymanager test.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: the ordering model was designed in TKT-XF5F (PLAN-ACI8);
this ticket adds read sorting and UI over it.

**Existing Solutions:**
- Order math: `internal/entitymanager/order.go` (MidpointOrder, Prepend/AppendOrder, NeedsRenumber, SortRelations); renumber in `manager_order.go`.
- Client midpoint math: `frontend/src/composables/useRelationReorder.ts` (used by `RelationCards.vue`). Not reused: the server computes the value.
- DnD library: `@atlaskit/pragmatic-drag-and-drop` already used by `useBoardDnd.ts`; its `hitbox` closest-edge addon gives drop position. Reuse rather than add vuedraggable/sortablejs.
- Page scope: `internal/dataentry/pagescope.go` already loads the anchor's edges; it discards the order values.
- Section traversal: `views.go` `traverseViewMany` keeps `byParent` edges in store order.
- Prior art: Linear/Jira "rank" (lexorank, server-side rank-before/after). We keep floats (existing scheme) and copy the before/after API shape.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Shared comparator (`internal/metamodel/order.go`, next to `FiniteOrder`):
`CompareRelationOrder(a, b)`: finite values first, then value, then the other
endpoint id, then tail. Used by the engine, list sort, section sort and
`sortRelationGroup`; mirrored in the client for optimistic reorder.

v1 is the OUTGOING side only: the anchor is the edge's source and the rows are
its targets (`_order_out`). Atlas's `bestaat_uit` tabs and section are this
case. Incoming-side ordering (faced sources, per-row permission) is a follow-up
ticket.

Engine (`internal/entitymanager`), no new Manager methods (plimsoll cap 37):
- `entity.RelationOptions` gains `Position *entity.OrderPosition`
(`{Before|After string, Step int}`, exactly one set). When set, `UpdateRelation`
resolves it INSIDE its existing `Store.Tx`: read the source's siblings, sort
with the comparator; if any sibling lacks a value or the target gap is
collapsed/tied, densify the side 1..N in current order first; then compute
Prepend/Append/Midpoint and write `_order_out`. Audit (move + densify) after
commit, like renumber. A Position combined with explicit properties is rejected.
- Pure helper `PlaceOrder(siblings, moved, pos) (value, densify bool, err)` in
`order.go`, table-tested on its own.
- Errors: `ErrOrderRefNotSibling` (mapped to the uniform 404), `ErrNotOrderable`.

API (`internal/dataentry`), package functions only (App cap 91, writeHandler
40):
- Existing `PATCH /api/v1/{plural}/{id}/relations/{rel}/{targetId}` accepts
`{"position": {"before": "<id>"} | {"after": "<id>"} | {"step": -1|1}}` as an
alternative to `meta` (both → 400). It runs the existing gate chain (path/peer
read gates, edge source, `relationMetaDenial` with the order key), then
UpdateRelation with Position. The ref must be readable via `visibleReader`; a
hidden ref and a non-sibling give the same 404 body. Answers 204.
- Page scope: `resolvePageScope` additionally returns, when the tab's
direction is outgoing and the relation is outgoing-orderable, the edge-value map
for the anchor's owned edges (already loaded). Orderability is read from the
same `state` snapshot. In `scopedSortedEntitiesScoped`, with no `sort` param,
sort by that map with the comparator instead of `applyV1Sorting`. Skip it
(property sort, not movable) when `_order_out` is hidden from the principal by a
relation `visible:` grant.
- Response `meta.relation_order: {relation, anchor, movable}` only when
relation order was applied. `movable` = ACL relation update with the anchor as
source AND `_order_out` writable (one check per request).
- Sections: a section is relation-ordered when it has no `sort:`, no
`group_by`, is flat, and its `collect_as` is written by exactly one
non-recursive `from: entry` rule over an outgoing-orderable relation, with no
`from: "*"` rule targeting it (decided from config). The traversal keeps a
separate `id → value` map for that rule (not touching `byParent`/`Parents`).
Section payload gains the same `relation_order`.

Frontend:
- `rela-components`: `useRowReorder` composable (pragmatic DnD
`draggable` with `dragHandle`, `dropTargetForElements`, closest edge from the
row's bounding box, no new dependency), Alt+↑/↓ on the handle, live region text.
Used by `RlTable` (`reorderable` prop, `reorder` event `{itemId, before|after}`)
and by EntityDetail's inline section table. `useBoardDnd`: cards become drop
targets; a drop reports `{column, before|after?}`.
- One helper `effectiveDefaultSort(listConfig, tabScope, schema)` replaces
the three `default_sort` sites (`listBaseParams`, `entityTarget`,
`useScopeNavigation`), so list, navigator and export walk one order.
- `listAllEntities` carries `meta.relation_order` from the first page.
- `EntityList.vue`: handles when `meta.relation_order.movable`, no reader
sort, not grouped. Optimistic move on the page; refetch on settle; toast and
rollback on error. Keyboard at a page edge uses `step`.
- `KanbanView.vue`: same-column drop → position PATCH; cross-column drop →
entity PATCH then position PATCH in ONE mutation, refetch after both. A failed
second write shows a toast; the status change stands. Other open clients see the
new order only on their next refetch (relation writes are not on the SSE entity
feed); documented.

**Alternatives rejected:**
- Separate `/move` route: duplicates PATCH's gate chain and missed the meta-field writability gate (RR, design review).
- Client-computed midpoint + existing PATCH (as RelationCards does): breaks at
page boundaries and under filters (neighbour not on screen), and races with
stale data. Server-side before/after avoids all three.
- Load whole scoped list unpaged when ordered: cost grows with the list.
- Opt-in per tab: user chose automatic.
- Lexorank strings: would change the TKT-XF5F storage format for no gain here.

**Files to modify:**
- internal/entitymanager/manager.go (UpdateRelation Position branch), manager_order.go, order.go (+ tests)
- internal/dataentry/pagescope.go, api_v1.go (sort + meta), write_handler.go (position body), views.go, sections.go (+ tests)
- internal/apiwire/v1/responses.go (RelationOrder meta, section field)
- internal/entity (RelationOptions.Position), internal/metamodel/order.go (comparator)
- frontend/packages/rela-components/src/components/table/RlTable*.vue, composables/useRowReorder.ts (new), useBoardDnd.ts
- frontend/src/utils/listParams.ts, composables/useScopeNavigation.ts, api/entities.ts (listAllEntities), components/entity/EntityDetail.vue
- frontend/src/api/relations.ts, components/lists/EntityList.vue, views/KanbanView.vue, types
- e2e/tests: new reorder spec
- docs/metamodel.md, docs/data-entry.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- URL ids and `position.before`/`after`: validated ids; unknown keys rejected; exactly one of before/after/step; step only -1 or 1; relation must be outgoing-orderable (else 400 `relation_not_orderable`); `position` with `meta` → 400.
- Relation type: must exist in the metamodel (else 404 as today).
- No order value is accepted from the client, so no float validation is needed on this path.

**Security-Sensitive Operations:**
- Write: the existing PATCH gate chain (read gates on path entity and peer, edge source/tail, `relationMetaDenial` on `_order_out`), then the engine's `RelationUpdateRequest` and `requireRelationFaceFor`. Densify writes only order values of the same source's edges, inside the same Tx, audited.
- Existence oracle: a hidden sibling or hidden moved edge endpoint answers the same 404 as a missing one (row-level rule in CLAUDE.md).
- Renumber may rewrite hidden siblings' order values: already the case in TKT-XF5F, values are not content, and the user learns nothing from it.
- Read: relation order is applied only when `_order_out` is visible to the principal, and after the ACL intersection; the meta only carries config (relation name, side) and the anchor id the client already sent.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1, AC5: `api_v1_reorder_test.go` (scoped list sort, page 2 boundary move); entitymanager `MoveRelation` table tests (before first, after last, middle, collapse → renumber).
- AC2, AC6: Playwright e2e on a prototype project with an orderable relation (row drag, keyboard move, kanban drop), reload asserts persistence.
- AC3, AC4, AC7 (UI): Vitest for EntityList/RlTable/section component.
- AC7 (server): sections test with and without `sort:`, multi-source traversal not ordered.
- AC8, AC9: API ACL tests with a restricted role.

**Edge Cases:**
- Edges without order values (pre-existing data, e.g. Atlas): shown after valued edges, by id; the first move densifies all siblings 1..N in that order, then places the row.
- Duplicate or collapsed values: comparator breaks ties; a move into a tie densifies first.
- Keyboard move at a page edge: `step` resolves the neighbour server-side.
- `_order_out` hidden by `visible:`: property sort, no handles.
- Section with `group_by` or `sort:`: not relation-ordered.
- Incoming-direction tab or section: unchanged behaviour (out of scope).
- Single-row list: handle shown, drop is a no-op.
- Drop on itself / same position: no request.
- Relation `orderable: both` or `outgoing` on an outgoing tab: `_order_out`.
- Faced types: edges owned by the anchor's face only (existing `edgesOwnedBy`).
- Filter or `q` active: drag still allowed; move is relative to the visible neighbour in the full order.

**Negative Tests:**
- Non-orderable relation → 400 `relation_not_orderable`.
- Zero or several of before/after/step, or step not ±1, or position with meta → 400.
- Affordance denying `_order_out` → 403 from the existing meta gate.
- ref == moved id → 400.
- Sibling not an edge of the anchor, or hidden → identical 404 body.
- No update permission → 403; hidden anchor → 404.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Row DnD in RlTable conflicts with row-link overlay / selection clicks. Mitigation: drag only from a dedicated handle cell (`dragHandle` option in pragmatic DnD).
- Kanban cross-column drop is two writes. Mitigation: one mutation, status first; on move failure refetch + toast; documented.
- Inline section table in EntityDetail: reorder via the shared composable, not by migrating to RlTable.
- Concurrent moves: serialized by `Store.Tx`; read-compute-write happens inside it.
- Plimsoll caps on Manager/App/writeHandler/viewsHandler: no new methods; package functions and an options field.
- Default-sort override surprises operators who set `default_sort` on a list used on an ordered tab. Mitigation: documented; reader can still sort by column.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md - `orderable` on relation types (missing today)
- [x] docs/data-entry.md - reorder on tabs, kanban, sections; default-sort override
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI changes)
- [x] ~~CLAUDE.md~~ (N/A: no new conventions)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 23 findings (3 critical, 8 significant, 9 minor, 3
nit), all addressed in this plan; linked as review-response entities on
TKT-RCRUWZ.
