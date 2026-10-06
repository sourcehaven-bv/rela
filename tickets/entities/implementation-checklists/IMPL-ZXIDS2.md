---
id: IMPL-ZXIDS2
type: implementation-checklist
title: 'Implementation: Document and e2e-test Create menu linking on entity pages'
started: "2026-10-06"
completed: "2026-10-06"
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
- `npx playwright test tests/space-create-page-link.spec.ts --repeat-each=5`: 15 passed.
- With `SpaceCreateMenu.vue` reverted to its state before BUG-PFLS22, the list-tab and timeline-tab tests fail (no relation found); the no-link test passes, as expected.
- spaces, pages and create-add-another specs: 18 passed.
- `just docs-check` passes after regenerating `docs/data-entry.md` from GUIDE-data-entry.
- Unit tests: not applicable, no application code changed. The e2e spec is the integration test.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
