---
id: IMPL-AP0WXO3
type: implementation-checklist
title: 'Implementation: GET /api/v1/_analyze runs thousands of statements per request on Postgres'
started: "2026-10-31"
completed: "2026-10-31"
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

Local copy of atlas (663 entities, 1,238 relations) on Postgres, built `-tags
postgres`, five `GET /api/v1/_analyze` runs per build and principal, access log
`queries` and `db_ms`:

| Principal | Build | Statements | Wall ms | DB ms |
|---|---|---|---|---|
| jvloothuis (5 roles) | develop | 12,058 | 3,550-14,550 | 3,140-13,450 |
| jvloothuis | this branch | 2,062 | 805-1,166 | 612-909 |
| wwarendorff (1 role) | develop | 6,210 | 1,518-3,294 | 1,303-2,952 |
| wwarendorff | this branch | 2,055 | 777-885 | 596-694 |

The machine was loaded by other work during the develop runs, so the wall times
of develop vary widely; the statement counts do not. The response body is
byte-identical to develop for both principals (`cmp`).

AC checks:

- Statement count no longer grows with the candidate rows:
TestQueryBudget_AnalyzeWithFieldRulesIsSizeIndependent is 26 reads at 10 and 50
tickets; with the page prime disabled it is 122 and 522.
- Pre-primed ctx (TKT-YLG695 order): TestQueryBudget_PushdownOnPrimedCtxAddsNoPrimeReads,
one read (the list) on a primed ctx.
- Violator and orphan re-load batched: TestQueryBudget_AnalyzeIsSizeIndependent
(9 reads at 10 and 50), cap and fault tests in analyze_validations_cap_test.go.
- No leak: TestACLAnalyze_* (hidden entity, hidden display property for orphans
and violators), TestAnalyze_FacedValidationViolators (same result as develop for
an every-face and a published-only principal).

`just ci` passes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
