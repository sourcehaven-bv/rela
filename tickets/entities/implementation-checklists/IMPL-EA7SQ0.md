---
id: IMPL-EA7SQ0
type: implementation-checklist
title: 'Implementation: Push down query scopes built from related() and equalities'
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

- `TestListPushdown_ScopeMatchesGoPath` drives the list API handler for four
principals, six scopes, four sort/filter shapes and three page windows, and
compares every served page and total with the Go path. It also asserts that
lowerable scopes never call `MatchingIDs` and that `not related(...)` stays on
the Go path. Mutating `listPlan.narrowed` to drop `Related` failed 116 subtests.
- `TestListPushdown_IncomingScopeKeepsRelationGate`: bob (editor-of gate in
`HasInbound`) gets exactly TKT-02, 04, 06.
- `TestListPushdown_DeniedTraversalIsEmptyWithoutAScan`: a denied traversal
answers an empty page with zero store reads.
- `TestQueryBudget_TraversalScopeIsPushedDown`: same call count at 10 and 50
rows, no `MatchingIDs`.
- `storetest.RunRelatedTests` passes on memstore, fsstore, sqlite and pg.
- `TestExplainPushedTraversalScopeListUsesDerivedIndexes` (pg): page and count
plans use the derived indexes, with no Seq Scan on entities.
- Full `go test ./...`, the postgres and sqlite suites, arch-lint, lint,
comment-lint, plimsoll and coverage-check pass. The one postgres failure,
`TestWebhookConflict_SchemaPinnedDSNIsIsolated`, needs `public.entities` in the
local test database and is unrelated.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
