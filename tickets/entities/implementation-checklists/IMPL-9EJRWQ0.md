---
id: IMPL-9EJRWQ0
type: implementation-checklist
title: 'Implementation: Page the data-entry collection reads'
started: "2026-11-13"
completed: "2026-11-13"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: internal read path; the budget and golden tests drive the full HTTP handler)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- AC1: `TestQueryBudget_PageScopedListIsSizeIndependent` (memstore and
postgres), `TestQueryBudget_IncomingOrderedListIsSizeIndependent` and the `?q=`
list budget test now assert rows over 100 and 1,000 padding entities.
- AC2: `TestScopedHeaders_ByIDsIsTheFullReadRestricted`,
`TestScopedHeaders_ByIDsKeepsTheWorldsFace` and the ByIDs rows of
`TestScopedHeaders_NarrowingsApplyToBothVerdictBranches`.
- AC3: `TestCommandExec_ListContextIsGatedAndRedacted`; the BUG-2TPO2F golden
files show the viewer payload losing hidden rows and values.
- AC4: `TestNoNewUnpagedReads` passes with the three helpers entries removed.
- `go test -race ./internal/dataentry/` passed; store packages
(graphquerynaive, memstore, fsstore, sqlitestore) pass after the naive
MatchingFaces change.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
