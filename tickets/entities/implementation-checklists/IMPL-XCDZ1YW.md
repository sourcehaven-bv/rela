---
id: IMPL-XCDZ1YW
type: implementation-checklist
title: 'Implementation: Role-relation roles count for menu and dashboard permission gates'
started: "2026-11-18"
completed: "2026-11-19"
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

- `TestHoldsPermissionOnAnyEntity` (internal/acl): direct edge, group edge, live
and expired conditional edge, conferred role without the permission, global
holder. The global-only `HoldsPermission` answer is checked unchanged in every
case.
- `ReadsOncePerRequest`: 3 store reads per Request for 20 rounds of checks plus
`PermitsWorld`; a global holder reads nothing.
- `ReadErrorFailsClosed` and `CanceledReadIsNotKept`: a store error counts no
conferred role; a ctx cancel is not memoised.
- `CeilingApplies`: a client baseline that withholds the permission wins.
- `TestGraph_HasOutgoingEdge` (internal/aclstore): faced tail, edge condition,
empty sources.
- `TestRoleRelationPermission_ShowsGatedUI` (internal/dataentry): nav entry,
dashboard card list, card value endpoint, space, page tab show for a
role-relation holder; the command authorizer and `HoldsPermission` stay
global-only.
- `TestMode_RoleRelationPermissionShowsUIOnly` (internal/access).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
