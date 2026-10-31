---
id: PLAN-NSYZIOI
type: planning-checklist
title: 'Planning: Card wall board with optional drag-to-group'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

The prototype on `proto/card-grouping` maps steps 5 (pre-mortem) and 6
(clustering) of the `dpia-sessie.html` mockup onto a kanban. The user tried it
and asked for three things: build it properly, make grouping optional, and make
groups more card-like (orderable, different colour).

**Scope:**

In scope:
- `layout: wall` on a kanban. Without columns it is one full-width grid of cards (a plain wall). With `columns_from` it is a pile of loose cards on top and group boxes below.
- `column_property` is no longer required when `layout: wall`.
- `card.style: note`: cards drawn as sticky notes, without the id.
- On a wall with `columns_from`: drop a card on the middle of another card to group them. A loose target creates a new group entity of the `columns_from` target type, offered by the page entity when `offered_by` is set. A grouped target means the card joins that group.
- Drop a card on the pile to ungroup it.
- Groups are drawn as cards with a solid, tinted background, distinct from the notes. `columns_from.style_from` colours a group per its enum value, as it colours columns today.
- Groups can be reordered by dragging their heading, when `offered_by` is set and that relation is `orderable: outgoing`. The move uses `moveRelation` on the anchor.
- Inline rename of a group from its heading.
- Cards inside a group and on the pile keep the reorder they have today.
- Animations: FLIP slide on move, pop-in of new cards and new groups only, a lifted drag preview, a wobble on the group target. All are off under `prefers-reduced-motion`.
- A board with one column fills the width and wraps its cards (all layouts).
- Config validation, docs, i18n (en + nl), unit, component and e2e tests.

Out of scope:
- Touch support (pragmatic-drag-and-drop has none).
- Keyboard grouping and keyboard group reorder (the card form edits the group relation).
- Deleting a group when it becomes empty. The empty box stays and accepts drops; it can be deleted from its detail page.
- Cards moving aside while dragging.
- A single atomic request for "create group". This takes up to four requests (see Risks).
- Anonymity and the other parts of the mockup.

**Acceptance Criteria:**
1. A kanban with `layout: wall` and no `column_property`/`columns_from` loads, shows all cards in one full-width grid, and, on a page tab scoped by an orderable relation, cards reorder by drag. Dropping on a card's middle does nothing beyond reorder.
2. On a wall with `columns_from`, dropping card A on loose card B creates a group titled after B, offered by the page entity, and both cards move into its box.
3. Dropping card C on a card in an existing group adds C to that group. No new group is created.
4. Dropping a grouped card on the pile removes its group relation.
5. Dragging group X's heading before group Y changes the order of the `offered_by` relation and the boxes show in the new order after reload.
6. Group boxes look like cards in a colour other than the notes, and `style_from` colours them.
7. Renaming a group in its heading saves the primary property. An empty name restores the old one.
8. A non-wall kanban with `columns_from` does not create groups on a middle drop (no behaviour change for existing boards).
9. Config errors: an unknown `layout` or `card.style` value, `swimlane_property` with `layout: wall`, and a missing `column_property` without `layout: wall` are refused at load.
10. Under `prefers-reduced-motion` no animation runs.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach was explored and tried with the user in the prototype)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Shopify Draggable and SortableJS: rejected. They move DOM nodes themselves, which fights Vue, would mean a second drag system next to `@atlaskit/pragmatic-drag-and-drop`, and have no grouping.
- pragmatic-drag-and-drop `setCustomNativeDragPreview` gives the lifted preview without a new dependency.
- Reused: `useRelationColumns` (columns from `columns_from` with `offered_by` order), `useListReorder` (card order), `moveRelation` in `frontend/src/api/entities.ts` (group order), `useDropTargetColumn`/`useDropTargetCard` in `useBoardDnd.ts`, the `heading` slot on `RlBoardColumn`.
- Prior art: TKT-KJ3Q07 (`columns_from`), FEAT-FE5P (reorderable relations), FEAT-006 (kanban).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Config (`internal/dataentryconfig`):
- `Kanban.Layout`: `""` (columns, default) or `wall`. `KanbanCard.Style`: `""` or `note`. Validated as enums.
- `validateKanbans`: `column_property is required` only when `layout` is not `wall`. A wall refuses `swimlane_property`.
- Remove the PROTOTYPE markers; document both fields in godoc and `docs/data-entry.md`.

Board library (`rela-components`):
- `useBoardDnd`: keep the `onto` placement and grid-aware `zoneAt`. Add an `allowOnto` option to `useDropTargetCard` so only grouping boards report the middle zone; others keep before/after.
- `useBoardDnd`: new `useDraggableSection`/`useDropTargetSection` pair for dragging a column by its heading, reporting `{sectionId, targetId, placement}`. Columns opt in with a `sectionsMovable` prop on `RlBoard`, which emits `move-section`.
- `RlBoardCard`: wobble only on `--drop-center`, disabled under reduced motion. `setLiftedPreview` skips rotation and scale under reduced motion.

SPA (`frontend/src`):
- Split the wall logic out of `KanbanView.vue` into `composables/useWallGrouping.ts` (groupOnto, ungroup, renameGroup, moveGroup), so the view only wires it. Each function reports errors through `uiStore.error` and refreshes the board.
- A plain wall builds one section with all cards; the view drops the heading when there is only one section and no grouping.
- `useRelationColumns`: refetch when a card points at a target that is not loaded (live updates from other readers).
- `useBoardFlip`: also pop in new `[data-section-id]` boxes (replaces the CSS `wall-box-open`, which ran for every box on load).
- Styles: notes and group cards use design tokens; add `--rl-color-note-*` tokens next to the existing colour tokens rather than hex values in the view.
- i18n keys for the pile heading, the rename label and the group-move failure, in en and nl.

**Alternatives considered:**
- A `group_on_drop` flag separate from `layout: wall`: rejected for now; grouping on drop is only meaningful on a wall, and a plain wall already covers "no grouping". Can be added later without breaking configs.
- A Lua action or a new endpoint to create a group atomically: rejected for this ticket; it needs a new server contract for a UI convenience. Revisit if partial groups show up in practice.

**Files to modify:**
- `internal/dataentryconfig/config.go`, `validate.go`, `validate_test.go` (or the kanban validation test file)
- `frontend/src/types/config.ts`
- `frontend/packages/rela-components/src/composables/useBoardDnd.ts` (+ test)
- `frontend/packages/rela-components/src/components/board/RlBoard.vue`, `RlBoardColumn.vue`, `RlBoardCard.vue`, `RlBoard.stories.ts`
- `frontend/src/composables/useRelationColumns.ts`, `useBoardFlip.ts`, new `useWallGrouping.ts`
- `frontend/src/views/KanbanView.vue`, new `KanbanView.wall.test.ts`
- `frontend/src/i18n` en + nl messages, design tokens file
- `e2e/tests/kanban-wall.spec.ts` + fixture project config
- `docs/data-entry.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Config keys `layout`, `card.style`: allowlisted enums, refused at load otherwise.
- Group titles typed by the user and derived from a card title: sent as a normal property write; the server validates them as for any form.

**Security-Sensitive Operations:**
- All writes go through the existing entity, relation and move endpoints, so ACL and validation apply unchanged. No new endpoint. A denied write shows the server's error and the board refreshes to the server state.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1: component test (plain wall renders one grid section; middle drop calls reorder only) + e2e (plain wall loads).
- AC2, AC3, AC4: component tests on `KanbanView.wall.test.ts` asserting the API calls; e2e for AC2 with a real drag.
- AC5: component test asserting `moveRelation(anchor, offered_by, X, {before: Y})`; e2e drag of a heading if the page object can drive it.
- AC6: component test on classes and `style_from` colour.
- AC7: component test for save and for empty-name restore.
- AC8: existing `KanbanView.columnsFrom.test.ts` plus a new case: middle drop on a non-wall board reorders.
- AC9: table-driven Go tests in the validate tests.
- AC10: unit test for `useBoardFlip` and `setLiftedPreview` with `matchMedia` mocked to reduce.
- `zoneAt`: unit tests for grid (x-based) and list (y-based) parents.

**Edge Cases:**
- Dropping a card on itself or on a card already in the same group: no write.
- Card title longer than 40 characters: group title cut with an ellipsis.
- Group type without a primary property: fall back to the first string property; if none, create without a title.
- `columns_from` without `offered_by`: groups are created but not offered; heading drag disabled.
- `offered_by` relation not orderable: heading drag disabled.
- Empty group: box stays and accepts drops.
- Reader without update rights: existing `canUpdate` check refuses before writing.

**Negative Tests:**
- Create group fails (server 4xx): error shown, board refetches, no box added.
- Second patch fails after the group exists: error shown, board shows the partial state from the server.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Partial group after a failure halfway the 2 to 4 requests. Mitigation: requests ordered so every partial state is valid and visible (an empty or one-card group), error shown, board refetched.
- `KanbanView.vue` is already large. Mitigation: the wall logic lives in `useWallGrouping.ts`.
- Heading drag conflicts with the rename input. Mitigation: a separate drag handle beside the input.
- The single-column change alters existing one-column kanbans. Accepted: requested by the user.

**Effort:** l

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - `layout: wall`, `card.style: note`, grouping behaviour, group reorder conditions

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**
1. (significant) Live updates miss new groups. The board subscribes live to the card list only; the group list (`useRelationColumns`) is fetched once. When another participant creates a group, their cards point at a group this reader has not loaded, so the cards fall back into the pile. Addressed: when a card's target is not among the loaded groups, `useRelationColumns` refetches once per unknown id; the live card update triggers that. Test: component test where a card arrives pointing at an unknown group.
2. (significant) Card reorder needs relation order. `useListReorder` only reorders on a page tab scoped by an `orderable` relation (`tabScope.relationOrdered`). A wall outside such a page cannot reorder cards. Addressed: AC1 and the docs state the condition; outside it cards show in list order and dragging only regroups.
3. (significant) Creating a group bypasses the create form, so a group type with other required properties fails with 422. Addressed: the server error is shown and nothing else is written (the create is the first request). Docs state that the group type should need only its title.
4. (minor) Grouping and group reorder are mouse-only. The keyboard path is the card's form (the `columns_from` relation field) and the existing keyboard move between columns. Recorded as out of scope.
5. (minor) Two readers dropping onto the same loose card at once create two groups. Accepted: both are visible and can be merged by dragging.
6. (nit) Group heading drag is silently unavailable when `offered_by` is not orderable. Addressed in docs.
