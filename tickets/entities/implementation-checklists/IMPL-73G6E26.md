---
id: IMPL-73G6E26
type: implementation-checklist
title: 'Implementation: Push list conditions and non-lowering scopes into the store'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (conditionlower_test.go, prefilter_test.go, queryscopelower_test.go)
- [x] Integration tests written (condition and nav status differentials over HTTP on memstore, sqlite and postgres)
- [x] Happy path implemented
- [x] Edge cases from planning handled (off-type values under `~=`, denied traversals, negated terms, rollups, `visible:`-gated attributes)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (newBudgetAppPadded, withConditionLists)
- [x] No hardcoded values in assertions when object is in scope (differentials compare against the Go path)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: server-internal read path with no UI change; covered by HTTP-level differentials)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (budget tests run against a temporarily disabled lowering)

**Verification Evidence:**

- Budget tests fail on the old path (rows at 100 vs 1000 padded): nav status 335 vs 3035, nav items condition 229 vs 2029, traversal scope 731 vs 1631, related condition 130 vs 1030, grant 132 vs 1032.
- Condition differential passes on memstore, sqlite and postgres 16; pgstore EXPLAIN test passes on PG16.
- TestGoldenParity passes on default and sqlite tags; no goldens regenerated.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (lowering shared through queryplan.LowerCondition)
- [x] No security issues introduced (rela-security-reviewer: one nit, addressed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind (TEMPOLDPATH patches reverted)
