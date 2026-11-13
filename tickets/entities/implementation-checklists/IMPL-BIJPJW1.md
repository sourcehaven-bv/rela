---
id: IMPL-BIJPJW1
type: implementation-checklist
title: 'Implementation: Multi-hop roll-up via'
started: "2026-11-13"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: exercised end to end through the data-entry list handler in TestRollups_PathPerPrincipal; no UI change)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: edge cases are pinned by storetest on all four backends)

**Verification Evidence:**
- storetest `Rollup/path` passes on memstore, fsstore, sqlitestore and pgstore:
  dedup across paths, far-end where, gated step, three hops, incoming steps,
  world per step, local-role predicates on a step, content-scoped edges.
- `TestRollups_PathPerPrincipal`: list values, filter and sort per reader
  through the real ACL. `TestQueryBudget_ListPathRollupsAreSizeIndependent`:
  8 extra reads at 10 and at 50 rows.
- `TestRollups_PathGatesEveryHop`, `TestCompile_PathRejects`, `TestRollupShape`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
