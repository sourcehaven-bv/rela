---
id: IMPL-AQFZ5DI
type: implementation-checklist
title: 'Implementation: Next-action suggestion grows with the data on postgres'
started: "2026-11-14"
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

tools/scalerun on postgres, perf project, 20 iterations (p50, statements):

| Principal | Scale 1 before | Scale 1 after | Scale 5 before | Scale 5 after |
| --- | ---: | ---: | ---: | ---: |
| manager | 27.6 ms, 12 | 3.5 ms, 9 | 188 ms, 34 | 1.9 ms, 9 |
| reader | 34 ms, 12 | 3.5 ms, 9 | 201 ms, 34 | 2.7 ms, 9 |

A traced request at scale 5 returned the same suggestion before and after.
Tests: `TestNextActionBudget_QuerySourceReadsAWindow` (fails without the
change: 111 vs 1011 rows), `TestFirstN_*`, `TestNextAction_*`,
`TestScopedHeaders_WindowWithScopeIsRefused`,
`TestGraphQueryExplainWindowReadsInIndexOrder` (fails without `, id`), the
pgstore suite with `-tags postgres`, `TestGoldenParity` on default and sqlite.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
