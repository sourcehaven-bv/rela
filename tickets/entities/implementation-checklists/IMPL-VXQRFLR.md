---
id: IMPL-VXQRFLR
type: implementation-checklist
title: 'Implementation: Flows M5a: compile enum transitions to headless flows'
started: "2026-11-12"
completed: "2026-11-12"
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

- `internal/statemachine/headless_test.go`: the compiled definition (states,
initial, finals), one branch per compiled move, two branches fail closed, unset
is not initial, a stranded value, guard before precondition (unheld and nil
guard), `flow.Gated` fails closed, a bind error fails only traversing edges, and
a 40-case cross-product that the `Performable` verdict matches the
`EnforceUpdate` error class.
- Race tests pass on `internal/statemachine`, `internal/entitymanager`,
`internal/affordances`, `internal/dataentry`, `internal/appbuild`,
`internal/mermaid`; existing tests are unchanged.
- CLI smoke run with a binary built from the branch on a scratch project:
open→done refused (undeclared), open→review allowed, review→done refused while
`when:` is false, allowed after linking `checked-by`, done→open refused, create
at `done` refused (illegal entry).
- `tickets/` passes cardinality and properties; validations reports only
the in-progress gate for this ticket and checklist. `docs-project/` and the
prototype projects with transitions load and report the same as the released
binary.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
