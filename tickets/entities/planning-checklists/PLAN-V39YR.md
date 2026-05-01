---
id: PLAN-V39YR
type: planning-checklist
title: 'Planning: Non-cards reverse relations silently ignored on data-entry forms'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In:
- Make the non-cards `RelationPicker` widget honour `field.direction: incoming`:
swap target-types source, swap cardinality property, fetch its own value list
via `getEntityRelations(..., 'incoming')`, and route saves through the same
direction-aware reconciler that `RelationCards` uses.
- e2e coverage for the non-cards path (list, add, remove).

Out:
- Changing the `entity.relations` payload shape on `GET /api/v1/{plural}/{id}`
(still outgoing-only by intent — see ticket "Proposed approach").
- Cards widget changes (already direction-aware).

**Acceptance Criteria:**

1. Non-cards picker with `direction: incoming` lists source entities — covered by
existing `reverse-relations.spec.ts` test "non-cards picker lists linked source
entities with direction: incoming".
2. Adding via picker persists as `(peer) --rel--> (current entity)` — needs new
e2e test.
3. Removing via picker deletes the underlying edge — needs new e2e test.
4. `targetTypes` resolves from `from:` and cardinality from `max_incoming` when
`direction: incoming` — implemented in `RelationPicker.vue:55-71`. Covered
indirectly by AC1 (target-types pulled correctly to render TASK-001).
5. e2e spec at `e2e/tests/reverse-relations.spec.ts` exists and passes for
non-cards case — exists; currently 4/4 pass.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- `RelationCards.vue` already implements the direction-aware pattern: it calls
`getEntityRelations(props.entityType, props.entityId, props.field.relation,
'incoming')` and resolves `targetTypes` from `relationType.from` when `direction
=== 'incoming'`. `RelationPicker.vue` mirrors this same pattern.
- `DynamicForm.vue` already has a save reconciler that handles `-incoming` and
`-outgoing` keyed pending changes; the picker change handler routes through it
via `updateIncomingPicker`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

The frontend code is already in place — `RelationPicker.vue` is direction-aware,
`DynamicForm.vue` bridges its `incoming-changed` event into the same pending-
changes map RelationCards uses, and the save loop strips `-incoming` and passes
direction through to `createRelation` / `deleteRelation`.

The remaining gap is e2e coverage: the existing spec covers list, but AC #2
(add) and AC #3 (remove) are not yet exercised by the non-cards picker. Add two
tests to `e2e/tests/reverse-relations.spec.ts` that:

1. Add — pick an unlinked task in the picker, save, then verify via API that
the new task's outgoing `implements` edge points at FEAT-001.
2. Remove — remove the seeded TASK-001 tile from the picker, save, then verify
via API that the `TASK-001 --implements--> FEAT-001` edge no longer exists.

Both need a second seeded task so the add test has a candidate to pick from
without colliding with TASK-001 (which the existing list test asserts is
rendered).

**Files to modify:**

- `e2e/tests/fixtures.ts` — add a second task entity (TASK-002) so the picker
add test has an unlinked candidate.
- `e2e/tests/reverse-relations.spec.ts` — two new tests for AC #2 and AC #3.
- `e2e/pages/form.page.ts` — small picker-interaction helpers (search input,
dropdown option click, remove tile button).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- The picker writes back through `createRelation` / `deleteRelation` with
`direction: 'incoming'`. The backend validates relation type, source/target
types, and cardinality on every write. No new untrusted input surface.

**Security-Sensitive Operations:**

- None — purely a client-side direction-routing change, no auth or filesystem.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1: list — existing passing test.
- AC2: add — new e2e test (open feature edit form, pick TASK-002 in
"Implemented by" picker, save, assert TASK-002 outgoing implements list contains
FEAT-001).
- AC3: remove — new e2e test (open feature edit form, click remove on
TASK-001 tile, save, assert TASK-001 outgoing implements list is empty).
- AC4: target-types/cardinality — exercised by AC1 rendering (the picker
resolves `from: [task]` correctly to find TASK-001).
- AC5: spec exists and passes — existing `reverse-relations.spec.ts`.

**Edge Cases:**

- Multi-cardinality: `implements` has no `max_incoming`, so adding TASK-002
alongside TASK-001 must be allowed (`isMulti.value === true`).

**Negative Tests:**

- Backend already covers cardinality / type-mismatch errors at the API layer
(no extra coverage needed in the SPA spec).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Low: the implementation is in place and behind a passing list-test. The new
add/remove tests will surface any bridge-path regression.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- N/A — bug fix on an existing form widget feature; no new user-facing surface.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: code already
implemented per ticket's own "Proposed approach"; only e2e test coverage remains
and it follows the existing pattern in this very file)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None — no design review run.
