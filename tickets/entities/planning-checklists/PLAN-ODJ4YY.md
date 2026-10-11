---
id: PLAN-ODJ4YY
type: planning-checklist
title: 'Planning: Reorder rows of a collection over the incoming side of an orderable relation'
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: the incoming side of every surface TKT-RCRUWZ built for the outgoing side.

- A list or board tab of an entity page scoped `{relation: X, direction: incoming}`.
- A `display: table` section whose rows come from one non-recursive `follow_incoming:` from `entry`.
- X must be `orderable: incoming` or `both`. The order is the anchor's `_order_in`.
- The move endpoint, the manager's move planner and the client's move call learn the incoming side.

Out:

- Tabs with two scope links, recursive traversals and `from: "*"` rules stay unordered, as on the outgoing side.
- The post-update renumber of `_order_in` after a direct `meta` write (`runRenumberAfterUpdate`) predates this ticket; see RR-IIQ4FH.

**Acceptance Criteria:**

1. A list or board tab scoped incoming over an incoming-orderable relation shows rows in the anchor's `_order_in` order, valueless edges last. Test: list API test with `scope_page`, `scope_tab` and `anchor`; `meta.relation_order.direction` is `incoming`.
2. Drag and keyboard moves on that tab persist after reload. Test: e2e drag and Alt+arrow on an incoming tab.
3. A table section with one non-recursive `follow_incoming:` from `entry` shows and moves in that order, but not with `sort:`, `group_by:` or nested display. Test: section API tests and an e2e section move.
4. Sort, grouping and swimlanes turn the order and the handles off. Test: `tabIsRelationOrdered` incoming cases; the EntityList and Kanban rules are shared with the outgoing side.
5. A principal who may not read `_order_in` sees no order. One who may not write it, or may not update every visible sibling edge, gets `movable: false` and 403 on a move. Test: API tests for an unreadable field, an unwritable field and an ACL that denies one sibling's source.
6. A move considers only readable edges; a hidden sibling ref is the uniform 404; a densify rewrites only readable edges. Test: manager tests with `Among`; handler test with a hidden source.
7. A source linking from two faces is addressed by the face of the place the list shows. Test: faced-source fixture; `relation_order.addresses` names `S@face` and the move by that address works.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: direct extension of the TKT-RCRUWZ design)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: same mechanism as the outgoing side)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `internal/dataentry/relation_order.go`: `newRelationOrdering` and `sectionOrdering`, outgoing only.
- `internal/dataentry/pagescope.go`: `resolvePageScope` already reads the incoming edges of an incoming tab.
- `internal/dataentry/views.go`: `EntryEdges` is recorded only for `follow:`; `traverseViewMany` already returns the edges of a `follow_incoming:` rule.
- `internal/dataentry/write_handler.go`: `handleV1UpdateRelation` already accepts `direction: "incoming"`, with the path entity as the target and the `{to}` segment as the source address (`S` or `S@face`, resolved by `incomingEdgeTail`).
- `internal/entitymanager/order.go`: `OrderKeyOf` and `SortRelations` already handle `_order_in` (the peer is the source).
- `internal/dataentry/search_linkable.go`: `linkablePage` batches source rows for per-row affordance and ACL checks.
- `frontend/src/utils/relationOrder.ts`: `tabIsRelationOrdered`, outgoing only.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Endpoint decision: reuse `PATCH /{plural}/{anchor}/relations/{rel}/{source}`
with `{"direction": "incoming", "position": {...}}`. The route already addresses
an incoming edge from its target this way, including the source's face
(`S@face`) and the `face_required` answer when a bare source links from two
faces. A new route would duplicate that address resolution and its gates. A
`side` field in `position` would repeat what `direction` already says. Before
and After name a sibling by its source address.

Server:

- `entity.OrderPosition.Among` becomes `[]RelationKey`, the edges the caller can read (RR-Y6O02Y).
- `moveRelation` takes the side from the request. Outgoing siblings stay the source's edges on the moved edge's tail. Incoming siblings are every edge into the target; the target side has no face.
- Incoming authorization (RR-1PWL4P): before the transaction, the manager authorizes an update of every candidate sibling, one `RelationUpdateRequest` per distinct source tail. Any denial fails the move with the ACL error. Inside the transaction it writes only the authorized edges, so an edge created in between is left alone.
- `insertionIndex` (RR-ULH7FY): on the incoming side the ref is a source address. A bare id matches the source's first place, which is where the list shows the row; `S@face` matches that tail.
- `newRelationOrdering` gets the side. Incoming keys are by source id on `_order_in`; the read probe and the movable gates use `_order_in`. Incoming edges are gated with `readableRelations` before the keys are built. `addresses` maps a row id to `S@face` when its shown place comes from a faced tail.
- Incoming movable (RR-PIA9DZ): the `_order_in` meta affordance and the ACL update for each distinct visible source tail, over source rows read in one batch as `linkablePage` does. A `storetest.Counting` test pins the cost at 10 and 50 rows.
- The anchor address carries its face whenever the anchor has one (RR-JQ9U0W).
- `resolvePageScope` builds the ordering for incoming tabs too. `views.go` records `EntryEdges` for `follow_incoming:` with the side; `sectionOrdering` passes it.
- `writeRelationPosition` accepts `incoming`. The order key and the error title follow the side. `visibleSiblingKeys` returns keys, listing `To: anchor` for incoming. For incoming it runs the meta affordance gate over every visible sibling source.
- Wire: `v1.RelationOrder` gains `direction` (`"incoming"`, omitted for outgoing) and `addresses` (omitted when empty).

Client:

- The `RelationOrder` type gains `direction?` and `addresses?`.
- `moveRelation` takes the order. It sends `direction: "incoming"` when set, and maps the moved row and a before or after row through `addresses`.
- `tabIsRelationOrdered` accepts an incoming link over an incoming-orderable relation.

Alternatives rejected:

- Authorize the anchor (target) for `_order_in` instead of each source: this changes who owns an edge in the ACL model, which is a policy decision outside this ticket.
- Authorize only the edges the plan rewrites: needs an ACL check inside the transaction or a re-plan loop.

**Files to modify:**

`internal/entity/writeapi.go`, `internal/entitymanager/manager_order.go`,
`internal/entitymanager/order.go`, `internal/dataentry/relation_position.go`,
`internal/dataentry/relation_order.go`, `internal/dataentry/pagescope.go`,
`internal/dataentry/views.go`, `internal/dataentry/search_linkable.go`,
`internal/apiwire/v1/responses.go`, `frontend/src/types/entity.ts`,
`frontend/src/api/entities.ts`, `frontend/src/composables/useListReorder.ts`,
`frontend/src/composables/useSectionReorder.ts`,
`frontend/src/utils/relationOrder.ts`, tests, the e2e relation-order spec and
the docs-project guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Path anchor and `{source}` address: gated reads, uniform 404 (existing).
- `direction`: only `incoming` changes behavior (existing).
- `position.before` and `position.after`: a source address. A hidden or non-sibling ref is the uniform 404.

**Security-Sensitive Operations:**

- A densify writes other sources' edges: each is authorized before the transaction, and only readable edges are candidates.
- The order shown on a read comes only from readable edges, and only when `_order_in` is readable.
- `addresses` names a face of a source that the reader can read at that face.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** each acceptance criterion above names its test.

**Edge Cases:**

- A source linking from two faces, one hidden: only the readable edge orders and moves.
- Valueless edges: the first move lands where dropped (densify).
- A step at the edge of a page is sent as a step; the server walks to the next sibling.
- A relation orderable only outgoing on an incoming tab: no order, and 400 on a move.

**Negative Tests:**

- A move with `direction: incoming` on an outgoing-only relation: 400 `relation_not_orderable`.
- Before a hidden source: 404.
- A principal who may update the moved edge but not a sibling's: 403, nothing written.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Changing `Among` to keys touches the outgoing path. The existing outgoing tests cover it.
- Movable on an incoming collection costs one batched source read plus one ACL check per distinct visible source. This is bounded by the anchor's incoming edges, which the scope reads anyway.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/metamodel.md (GUIDE-metamodel): the incoming side is shown.
- [x] docs/data-entry.md (GUIDE-data-entry): incoming tabs and sections.
- [x] docs/data-entry/api-reference.md: `direction: incoming` with `position`, and `relation_order.direction` and `addresses`.

## Design Review

- [x] Run `/design-review` before starting implementation (inline review against the cranky-code-reviewer and rela-security-reviewer criteria; this run cannot start agents)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-PIA9DZ, RR-1PWL4P, RR-Y6O02Y (significant,
addressed in plan); RR-JQ9U0W, RR-ULH7FY (minor, addressed in plan); RR-IIQ4FH
(minor, deferred); RR-2HX7I3 (nit, wont-fix).
