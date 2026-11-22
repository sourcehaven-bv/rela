---
id: IMPL-UX4P9T4
type: implementation-checklist
title: 'Implementation: Phone list titles, laptop table, banner placement and phone Overview header'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (columnSizing, tableColumns, DashboardView, ReadOnlyBanner.placement)
- [x] Integration tests written (e2e mobile-layout: phone title width and Overview header; e2e list: laptop table)
- [x] Happy path implemented
- [x] Edge cases from planning handled (no kept columns, kept cells that fit beside the title, a column with its own minimum)
- [x] Error handling in place (N/A: layout only)

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
A local server built from the branch: at 375px invoice titles get their own line and "Download PDF" shows in full; at 1024px and 1440px the full table shows with aligned headers; the phone Overview shows its title beside the menu toggle above the notice. The banner order is covered by a unit test only (it needs tenant mode). Full vitest, vue-tsc, eslint and the touched e2e specs pass.

## Quality

- [x] Code follows project patterns (check similar code) (sizes passed as custom properties, as before)
- [x] Checked for DRY opportunities (header and rows share columnSizing.ts)
- [x] No security issues introduced (CSS and layout only)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
