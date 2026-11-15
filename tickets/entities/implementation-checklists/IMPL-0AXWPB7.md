---
id: IMPL-0AXWPB7
type: implementation-checklist
title: 'Implementation: Page executeQuery and the remaining scopedHeaders callers'
started: "2026-11-14"
completed: "2026-11-14"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: read-path change with no UI; the handler-level differential and budget tests drive the real HTTP handlers)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `TestSearchPlan_MatchesGoPath` and `TestSearchPlan_PositionMatchesGoPath` pass on memstore and on postgres 16 (`-tags postgres`). The postgres run found the pgstore position ignoring the enum rank; fixed, and the new storetest ranked cases fail without the fix.
- `TestQueryBudget_SearchLimitIsSizeIndependent` fails when the plan is forced off (rows grow 111 to 1011).
- Golden parity passes on the default build and with `-tags sqlite`, without regeneration.
- Mutation checks: removing OrderBy, Props or the ACL query each fails the differential test.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
