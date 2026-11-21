---
id: IMPL-2XXTRKN
type: implementation-checklist
title: 'Implementation: Sankey view for data-entry (sankeys: config, layout, view)'
started: "2026-11-21"
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
Ran a demo project (`.ignored/sankey-demo`) with the `flow` and `pips` sankeys in the browser pane. Checked ribbons, node selection, the detail panel, scroll-into-view of a clicked node, pip dots and `+n` with the panel open and closed. Go tests (`dataentry`, `dataentryconfig`), vitest (`SankeyView`, i18n guards), `vue-tsc`, eslint, prettier, golangci-lint on the three Go packages, `just arch-lint`, `just plimsoll` and `just comment-lint` pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
