---
id: IMPL-ONGLWB
type: implementation-checklist
title: 'Implementation: Create sets a hardcoded status when the schema declares no default'
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

- CLI on a scratch project: `note` (no status property) is created without
`status`; `task` with an enum status and no default is created without `status`;
after adding `default: done` to the property, `task` gets `status: done`.
- Go tests on every create path fail on the unfixed code (stray `draft`,
first enum value, and a state machine create rejected with `illegal entry
state`) and pass with the fix.
- E2E form create: a decision gets no status; a feature with the status left
empty gets the type default `draft`. The first case fails on the unfixed server
with `Received: "draft"`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
