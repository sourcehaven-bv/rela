---
id: PLAN-3ZI6P98
type: planning-checklist
title: 'Planning: Manual list order'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- `lists.<id>.order: manual` in data-entry.yaml.
- The `internal/listorder` package: an ordered id list per (owner, list) on `state.KV`.
- Applying positions in the list pipeline.
- A move endpoint.
- Drag handles in EntityList for manual lists, flat or grouped. A drag stays inside its group.
- Dragging inside groups on relation-ordered tabs.

Out of scope:
- Shared (team) order.
- Cross-group drags, which would change a property.
- Manual order on dashboards, kanban and views.
- A store-native postgres table.

**Acceptance Criteria:**
1. A list with `order: manual` shows drag handles. A drag moves the row, and the order survives a reload. Test: an API test moves a row and lists the page again.
2. Rows without a position follow the list sort, after the positioned rows. Test: a pipeline unit test.
3. Each user has their own order. Test: alice's move does not change bob's page.
4. Positions apply only to rows that pass the row gate. A hidden row never takes a slot, and the response leaks no hidden id. Test: an ACL test with a hidden positioned row.
5. A move drops the positions of rows that are no longer in the list. Test: a row leaves the list, and the next move removes it from storage.
6. A reader's own sort turns manual order off for that read. Test: `?sort=` ignores positions.
7. Grouped lists: a drag stays inside its group and keeps its place after reload. Grouped relation tabs allow dragging inside a group. Test: vitest with `planRowMove` on section rows, and an API test.
8. The config refuses `order:` values other than `manual`, and refuses `order: manual` together with `relation_order`. Test: a dataentryconfig table. `order: manual` with `group_by.relation` or `group_by.buckets` is refused too.
9. A scoped tab shown in relation order keeps the relation order; manual order is off there (`meta.manual_order` false).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: the piles, lease and userstate patterns already cover per-user state outside the graph)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `internal/userstate/kvuserstate` and `internal/piles`: per-user state on `state.KV`, kept outside store.Store.
- `useListReorder.ts`, `planRowMove` and `createMoveQueue`: optimistic drag with a move queue.
- `relation_order.go`: server-side ordering of a page.
- Fractional sort keys were considered. Rejected: an ordered id list is simpler at a cap of 500, and one document per (owner, list) is one KV read per page.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **`internal/listorder`**
   - `Service{kv state.KV}`. `Order(ctx, list) ([]string, error)` reads the caller's ids. `Save(ctx, list, ids []string) error` writes them.
   - The owner is resolved from the stamped principal, as in `piles.OwnerFrom`. A request without an owner gets `ErrNoOwner`.
   - The key is `listorder/<sha256(owner)>/<sha256(list)>`, so a user name or list id never shapes a path.
   - The value is JSON `{"ids":[...]}`, capped at `MaxIDs=500`. Each id must be at most 200 bytes with no control characters. Longer lists are cut from the tail.
   - An in-process mutex per key serializes writes. Across nodes on postgres, the last writer wins. This is acceptable because only the owner writes their own order.
   - Pure helper `Apply(rows []string, ids []string) []int` gives the stable permutation: positioned rows first, in stored order, then the rest in their incoming order.
   - Pure helper `Move(full []string, stored []string, id, before, after string) ([]string, error)` returns the new stored ids. The full order is the current page order with positions applied. The helper removes `id`, inserts it next to the neighbour, and stores the prefix through the moved row plus any stored ids still in `full`. Ids that are not in `full` are dropped; this is the GC.
2. **Config.** `List.Order string` (`yaml:"order"`). It is validated: `manual` only, and not on a list with `relation_order`/scoped relation ordering. It is classified as editable in configedit.
3. **Pipeline.** In `listPage`, when the list in `list_id` has `order: manual`, the request carries `manual_order=1` (the SPA sends it while the reader has not sorted; a grouped list always sends its group sort, so `sort` cannot be the signal), and the service is wired:
   - skip the pushdown plan;
   - after `applyViewCondition`, permute `all` with the stored order. On a grouped list the permutation runs inside each run of equal group values, so the sections keep their order. `group_by.relation` and `group_by.buckets` are refused together with `order: manual`;
   - then page.
The gated `all` is the only input, so a hidden row never takes a slot. The
response gains `meta.manual_order: true`.
4. **Endpoint.** `POST /api/v1/_lists/{list}/order` with body `{id, before|after}`.
   - The list must exist and be manual (404/400 otherwise).
   - The endpoint runs the list pipeline with the list's own base query and no paging, then calls `listorder.Move`, then `Save`.
   - It returns 204. An id or neighbour that is not in the gated rows returns 404, the same as a missing row.
   - The handler is a package function, because App is at its plimsoll cap.
5. **Wiring.** `dataentry.SetListOrder(app, svc)` runs from `dataentrywire.wireOptionalStores` with `svc.State()`. A nil KV leaves the feature off; the endpoint then answers 404 and lists use their sort.
6. **SPA.**
   - `useManualReorder` mirrors `useListReorder`. It is active when `meta.manual_order` is set and the reader has not sorted. It plans on the section's rows when the list is grouped, and refuses a target in another section. The move is optimistic, then refetches.
   - Grouped relation tabs: `useListReorder` takes the same section rows, so `active` no longer requires `!grouped`.

**Files to modify:**
- internal/listorder/{listorder.go,listorder_test.go}
- internal/dataentryconfig/{config.go,validate.go,manual_order_test.go}
- internal/configedit/allowlist.go
- internal/dataentry/{list_pipeline.go,manual_order.go,manual_order_test.go,api_v1 routes,livepath/requeststats if needed}
- internal/dataentrywire/wire.go
- frontend: composables/useManualReorder.ts(+test), useListReorder.ts, EntityList.vue, api/entities.ts, types
- docs: GUIDE-data-entry, api-reference

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- The list id in the path must be a configured list. An unknown id gets 404.
- The body ids `id`, `before` and `after` must be among the gated rows of that list. Otherwise the answer is 404, the same as a missing row, so a hidden row is not confirmed.
- The config value `order` is allowlisted (`manual`).

**Security-Sensitive Operations:**
- KV keys are hashes, so neither the user name nor the list id shapes a path.
- Positions are read and written only for the stamped principal. No API reads another user's order.
- Stored ids are applied only to gated rows. A hidden row that was positioned before stays invisible, and its id never leaves storage.
- Every read and move runs the full list pipeline, including the ACL. Cost: one KV read per page and one pipeline run per move.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** listed in the acceptance criteria, as API tests in dataentry
on the memory store.

**Edge Cases:**
- An empty stored order.
- A stored id that was deleted.
- A move onto itself.
- The neighbour is the last row.
- The cap of 500 is reached.
- Two moves running at once.
- A corrupt KV value is treated as an empty order and logged.

**Negative Tests:**
- An unknown list.
- A list that is not manual.
- A hidden id.
- No owner.
- `before` and `after` both set.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Skipping pushdown on manual lists costs a full in-memory load. That is already the default path, and manual lists are task-sized.
- A last-writer-wins race across nodes for one user. Accepted.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-data-entry (`order: manual`), api-reference
(move endpoint, `meta.manual_order`).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the linked review responses.
