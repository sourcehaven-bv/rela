---
id: IMPL-8YARL46
type: implementation-checklist
title: 'Implementation: Edit owned and linked items inline on the parent page'
started: "2026-11-03"
completed: "2026-11-03"
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

- Go: `sections_editable_table_test.go`, `editable_table_test.go`,
`TestQueryBudget_ViewEditableTableAddsNoStoreReads`; `go test` on dataentry,
dataentryconfig, apiwire and configedit passes.
- SPA: `EditableRowCells.test.ts` (editable vs read-only cell, unwritable
verdict shows display, typed value from `_props`, numeric class, save on the row
address), `sectionTableRows.test.ts` (row count over groups, cap, index and
apply for flat and group rows, owner mismatch), `sectionTotals.test.ts`. `vitest
run src/components/forms src/components/entity`: 108 files, 1785 tests pass.
Typecheck clean.
- e2e: `table-inline-edit.spec.ts` 3/3 (input vs plain column, edit persists
after reload, roll-up total follows an edit without reload); related specs
89/89.
- Browser (e2e fixture server): invoice with two charges; edited qty 3 to 10,
the totals line went from 5 to 12 without a reload. Found and fixed numeric
cells rendering left-aligned (`InlinePropertyValue` numeric now `align-items:
flex-end`); rechecked after rebuild.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
