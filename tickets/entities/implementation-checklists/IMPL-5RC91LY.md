---
id: IMPL-5RC91LY
type: implementation-checklist
title: 'Implementation: Dashboard: one card per row on phones'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: CSS only; jsdom has no layout, so the check is the e2e spec)
- [x] Integration tests written: `e2e/tests/dashboard-card-width.spec.ts` asserts one track and stacked full-row cards at 400px
- [x] Happy path implemented
- [x] Edge cases from planning handled (wide and full cards at 400px; no overflow)
- [x] ~~Error handling in place~~ (N/A: CSS only)

## Test Quality

- [x] Uses the spec's existing named dashboard fixture
- [x] Assertions compare against measured grid values, not literals
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolation)
- [x] Property comparisons use the measured geometry object

## Manual Verification

- [x] Feature tested end-to-end in Playwright (Chromium, built server)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases verified

**Verification Evidence:** The width spec passes (5 tests at 1440, 900, 700, 375
and 400px) and dashboard.spec.ts passes. With the CSS change reverted, the 400px
test fails: two tracks of 178px instead of one. DashboardView vitest: 27 passed.

## Quality

- [x] Code follows project patterns
- [x] Checked for DRY opportunities: the change removes two rules
- [x] No security issues introduced
- [x] ~~No silent failures~~ (N/A: CSS only)
- [x] No debug code left behind
