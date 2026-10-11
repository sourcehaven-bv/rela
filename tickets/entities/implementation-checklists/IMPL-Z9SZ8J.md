---
id: IMPL-Z9SZ8J
type: implementation-checklist
title: 'Implementation: Reorder rows of a collection over the incoming side of an orderable relation'
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Manager: `internal/entitymanager/move_incoming_test.go` covers moves and
steps on the incoming side, errors (outgoing-only relation, unknown sibling, bad
address, self ref), the sibling authorization (one denied source fails the move
and writes nothing; `Among` without it succeeds and leaves its edge alone), and
`id@face` refs in `PlaceOrder`.
- API: `internal/dataentry/relation_order_incoming_test.go` covers the
incoming list tab (order, paging, reader sort wins, other side only), movable
false for one denied source and for a read-only `_order_in`, a hidden
`_order_in` turning the order off, moves through the PATCH route, 400s, the 403
naming `meta-read-only:has-step._order_in`, the uniform 404 for a hidden
sibling, a step that ignores hidden siblings, the incoming table section, and
faced sources (`addresses`, a move on `POL-1@published` leaving the hidden draft
edge alone).
- Cost: `TestQueryBudget_IncomingOrderedListIsSizeIndependent` pins 10 reads
at both 10 and 50 rows.
- Frontend: vitest for `tabIsRelationOrdered` on incoming links,
`orderMoveArgs` addresses and direction, and `useListReorder` sending an
incoming move. Full vitest suite: 265 files, 3861 tests pass.
- E2E: `relation-order.spec.ts` "an incoming list tab moves a source" drags
and steps a feature on a task's Features tab and checks the order after a
reload. All 5 tests in the spec pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
