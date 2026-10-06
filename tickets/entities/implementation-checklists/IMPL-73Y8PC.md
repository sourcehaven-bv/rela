---
id: IMPL-73Y8PC
type: implementation-checklist
title: 'Implementation: Cardinality analysis: batch relation counts and count visible edges only'
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

- `rela analyze cardinality` on `tickets/` and `docs-project/` with the
  new binary: all constraints satisfied, as before.
- AC1: `TestCheckCardinality_ReadBudget` reads the same number of times at
  10 and 50 subjects over `storetest.Counting`.
- AC2: `TestAnalyze_FacedTypes/cardinality_counts_visible_edges_only`
  (data-entry: POL-1 reports "has 0" when FEAT-1 is hidden) and
  `TestAnalyzeCardinality_CountsVisibleEdgesOnly` (MCP: no violation and
  no mention of the hidden control).
- AC3: the CLI passes `schema.Ungated(store)` to the same
  `schema.CheckCardinality`; the existing CLI and analysis tests pass
  unchanged apart from the injected-error double, which now fails
  `ListRelations`.
- Gate fault: `TestScriptReader_ListRelationsStrictReturnsGateFaults`
  shows the strict read returns the fault the tolerant read hides.
- Edge cases: per-face counting (`cardinality per face`), max bound 0 (MCP
  test), edge read error (`TestCheckCardinality_EdgeReadErrorAborts`).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
