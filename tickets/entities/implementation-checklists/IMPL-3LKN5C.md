---
id: IMPL-3LKN5C
type: implementation-checklist
title: 'Implementation: Matrix card: full integer axis range, axis titles and order; schema min/max on integers'
started: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Unit: `internal/metamodel/range_test.go` (load errors, value checks, InRange),
`TestMatrixAxisTooWide`, `TestDashboardCard_MatrixAxisOptions`,
`TestFoldMatrix_IntegerAxisRange` (declared, observed fill, one bound, no data,
out of range, non-integer, desc), `TestFoldMatrix_IntegerAxisIsCapped`,
`TestFoldMatrix_HiddenValueDoesNotWidenAxis`, `TestFoldMatrix_AxisTitles`,
MCP `tools_schema_range_test.go`, SPA `DashboardMatrix.test.ts` and
`widgets.test.ts` (NumberWidget min/max).
Integration: `api_v1_range_test.go` (schema API carries min/max; PATCH out of
range gives 200 with `property_value_invalid`), `TestDashboardCard_Displays`
matrix subtest through the card endpoint.

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

Branch binaries (sqlite build) against a backup copy of the grc-gemeente demo
in `.ignored/demo`, with `min: 1, max: 5` on `kans`/`impact` and
`rows_order: desc` plus axis labels on the Risicomatrix card.

- AC1/AC9: `rela validate` and the three analyze gates pass on the copy; load
  errors covered by unit tests.
- AC2: `USER=PERS-CISO rela update RIS-001 -P kans=7` printed
  `WARNING: property_value_invalid at /properties/kans: 7 is out of range (must
  be from 1 to 5)`; `rela analyze properties` then listed RIS-001 with the same
  message. Value restored to 3.
- AC3: `/api/v1/_schema` returned `"kans":{...,"min":1,"max":5}`.
- AC4/AC7/AC8: card endpoint returned rows 5..1, columns 1..5, `row_title`
  "Impact", `column_title` "Kans". A headless Chromium screenshot of the
  dashboard showed the full 5x5 grid with empty row 1 and columns 1 and 5, the
  column title above the values and the row title beside them.
- AC5/AC6: covered by fold table tests.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`MatrixAxisTooWide` is shared by config load and the fold. The fold still runs
on gated, redacted rows; a hidden value marks the card partial and never widens
an axis.
