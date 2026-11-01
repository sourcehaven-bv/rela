---
id: IMPL-1UK2YVA
type: implementation-checklist
title: 'Implementation: Dashboard cards render values like the rest of the app'
started: "2026-11-01"
completed: "2026-11-01"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (DashboardView.test.ts "value rendering", denseCell.test.ts, styleColors.test.ts styleBarColor)
- [x] Integration tests written (test full flow, not just units) (e2e/tests/dashboard-card-rendering.spec.ts against rela-server)
- [x] Happy path implemented
- [x] Edge cases from planning handled (value outside the enum, unset value, non-enum group_by, unstyled value, empty cell, unknown property, redacted cell, per-row type)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: pure rendering; no new error paths. Unknown properties fall back to the plain text the card showed before)

## Test Quality

- [x] Using fixture builders or factories for test data (card(), deal(), seedDeals(), searchReturns())
- [x] No hardcoded values in assertions when object is in scope (amount and date expectations come from formatAmount/formatDate)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded (e2e link uses the created entity id)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Screenshot of the e2e "rendering" dashboard
(rela-server, locale nl-NL): breakdown shows badges Nieuw (blue), Bezig
(orange), Klaar (green) in declared order although Bezig has the most rows; bars
take blue, amber and green. The wide table card shows a title link, a "Bezig"
badge, "€ 3.500" and "1 mrt 2026". All four dashboard e2e specs pass (17 tests).
AC5: grep finds no map-form `sort:` left in docs-project or docs.

## Quality

- [x] Code follows project patterns (check similar code) (same dense routing, registry and formatCellValue as EntityList; Badge for breakdown labels)
- [x] Checked for DRY opportunities (dense cell resolution extracted to utils/denseCell.ts; card derivations moved to components/dashboard/cardValues.ts, which also shrinks DashboardView)
- [x] No security issues introduced (labels interpolated as text; style classes pass styleSwatch's allowlist)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no new failure paths)
- [x] No debug code left behind
