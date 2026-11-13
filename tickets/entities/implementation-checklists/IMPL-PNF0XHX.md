---
id: IMPL-PNF0XHX
type: implementation-checklist
title: 'Implementation: Workshop step mode for pages (mode: steps, present mode)'
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

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Automated: Go table tests for mode/prompt validation (page_test.go) and the
sidebar wire (pages_test.go); 11 PageView unit tests for steps, prompt,
Back/Next, present mode following the route, unmount, tabs page ignoring
?present=1, and PageDown/PageUp (present only, not in a text field); a
Presenting story play test on RlAppShell; e2e "a steps page walks its steps, and
presents without the sidebar" in pages.spec.ts. All pass (pages e2e 9/9,
frontend unit 1342/1342 in views/components/pages/composables).

Manual, rela-server on a copy of the tickets project with a three-step page
(list, ticket board, idea board):
- AC1/AC3: steps numbered, current marked, no tab bar; prompt shown large.
- AC2: Back disabled on step 1, Next disabled on step 3; history back walks
the steps.
- AC4: Present hides the sidebar and suggestion card and zooms the view;
?present=1 kept by step links and PageDown; Exit removes it; browser back out of
?present=1 restores the sidebar.
- AC5: PageDown moves to the next step while presenting; does nothing
outside present mode.
- RR-MJGVME2: dragged IDEA-HUWQ from Exploring to Validated on the zoomed
board; it landed in the right column and saved (status: validated).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
