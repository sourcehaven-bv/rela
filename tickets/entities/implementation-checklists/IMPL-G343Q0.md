---
id: IMPL-G343Q0
type: implementation-checklist
title: 'Implementation: Enforce relation cardinality at write time and add an atomic replace operation'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: internal/entitymanager/cardinality_test.go and copy_cardinality_test.go (CreateRelation, ReplaceRelations, cascade, copy)
- [x] Integration tests written (test full flow, not just units): internal/dataentry/relations_cardinality_test.go (PATCH and POST through the handler) and caldav_move_test.go; pgstore relation_version tests
- [x] Happy path implemented: ReplaceRelations re-points a bounded relation in one transaction from either side
- [x] Edge cases from planning handled: all edge cases from the plan have a test
- [x] Error handling in place (errors surfaced, not swallowed): typed errors CardinalityError (with Key), InvalidRelationError, RelationCreateError and ErrRelationAlreadyExists map to 422, 409 or 403

## Test Quality

- [x] Using fixture builders or factories for test data: shared manager fixtures (cardinalityManager, cardinalityManagerWith, cardinalityManagerACL) over a small task/status schema
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: on the demo server a relation field re-point leaves exactly one `heeft_status` edge, and a kanban drag replaces the edge
- [x] Each acceptance criterion verified with test scenario from planning: AC1 to AC3 by the tests named in the plan
- [x] Edge cases manually verified: CalDAV edge cases are covered by handler tests, not by a live CalDAV client

**Verification Evidence:**
- Demo: changing Status on a task re-points the edge and leaves exactly one edge.
- `go test ./internal/...` passes.
- Frontend vitest full suite passes: 3770 tests in 255 files. `vue-tsc` is clean.
- `golangci-lint` reports 0 issues on the changed packages.
- Postgres tests (pgstore, entitymanager with `-tags postgres`) pass against a throwaway database.

## Quality

- [x] Code follows project patterns (check similar code): bounds live next to the existing entitymanager write checks; handler errors use the existing problem responses
- [x] Checked for DRY opportunities: one capacity check shared by create, cascade, copy and replace
- [x] No security issues introduced: security review finding on 93f44f514 fixed; removes are limited to named edges with per-edge ACL
- [x] No silent failures (errors logged AND returned): the disallowed-type fallback logs a warning and returns it to the client
- [x] No debug code left behind
