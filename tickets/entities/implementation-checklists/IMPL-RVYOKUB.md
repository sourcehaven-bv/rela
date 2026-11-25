---
id: IMPL-RVYOKUB
type: implementation-checklist
title: 'Implementation: Natural-language date input'
started: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units): InlinePropertyValue test mounts the widget inside RlInlineEdit
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: inputs are short phrases and a fixed today)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: expected dates are the point of each case)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
rela-server on a demo project, Dutch interface, create form: `volgende vrijdag` + Enter set the picker to 27-11-2026 and cleared the box; `next banana` + Enter marked the box invalid, showed the Dutch message, kept the date and did not submit the form. vitest: naturalDate (52), DateWidget (6), InlinePropertyValue (21) pass; the InlinePropertyValue case fails without the RlInlineEdit change.
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned): an unparsed phrase is shown as invalid with a message
- [x] No debug code left behind
