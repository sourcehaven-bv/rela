---
id: IMPL-YIXKCDP
type: implementation-checklist
title: 'Implementation: Finish next actions: every page surface, context-aware sources, per-view and global placement config'
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

- Ran rela-server on the e2e fixture with a banner band, a `visible_on: [list, kanban]` query source and a `context: feature` source. Screenshots at 1280x800 and 390x800 (kept in `.ignored/na-shots/`):
  - The card appears on the features list and above the feature board. All columns stay in view, and the card lines up with the page gutters.
  - On `/entity/feature/FEAT-001` the context source shows "This one: User Authentication". On FEAT-003 it follows that entity.
  - Nothing appears on the dashboard (not in `visible_on`) or on search (`next_action_surfaces: {search: false}`).
  - An in-app navigation from the list to the board keeps the same suggestion after its impression POST.
- `e2e/tests/next-action-surfaces.spec.ts` automates these four scenarios.
- Go handler tests cover:
  - A hidden entity's answer is byte-identical to a nonexistent id's.
  - A faced address resolves by its bare id.
  - Another type's id gives no candidate.
  - Unknown surface → 400. Unknown view → quiet 200.
  - A disabled page does 0 store reads, and a context source reads the same at 10 and 50 rows.
  - The real matcher evaluates the condition on the context candidate.
  - A visible:-hidden property never reaches a context suggestion's message.
  - "Seen it" hides a context suggestion; "shown" does not.
  - A request without a surface follows the dashboard switches.
- Frontend unit tests cover the per-page memory of `current`, the debounced refresh on `context_type`, and the retry after a failed load. The per-page memory test fails when that memory is removed.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
