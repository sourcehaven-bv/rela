---
id: IMPL-1TPHYDF
type: implementation-checklist
title: 'Implementation: Quick entry form with a global hotkey'
started: "2026-11-24"
completed: "2026-11-24"
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

Ran rela-server on a copy of examples/todo-app with `quick_entry: {form:
quick_todo, list: today}`. `/_config` served the block. Pressing `q` on the
dashboard opened `/quick`. Saving "QE check item" wrote a todo with `when:
today` (the list's assign), showed the "saved" toast and opened an empty form.
Escape returned to the dashboard. The desktop hotkey is covered by unit tests
with fake registrar and windows (switch closes popup, stale popup closed,
concurrent presses open one window, under -race); the native global shortcut
itself was not pressed by hand.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
