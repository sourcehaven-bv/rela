---
id: IMPL-NEEKMFL
type: implementation-checklist
title: 'Implementation: Flows: step form from a data-entry form instead of writes:'
started: "2026-11-19"
completed: "2026-11-19"
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

- `e2e/tests/flows.spec.ts` against a built rela-server: the collect step
renders the `collect_documents` form, finds its checkbox by the form's label
("Documents received", not the schema label), completes the step and finishes
the run. Passed.
- `TestResolveStepForms` covers an undeclared form, another type, a wizard,
relations, body, undeclared, derived and reserved properties, and hidden, blank
and repeated fields.
- `TestParse_Form` covers a form on a state without `waiting_for:`, a final
state, surrounding spaces, and that `writes:` is now an unknown key.
- `TestCheckTriggerACL` covers a trigger setting a step-form property, one
setting a property the form does not edit, and a data-entry edit that adds a
locked field under an unchanged trigger.
- `TestPreview_TriggerCheckInput`: a data-entry-only Configure edit is judged
when flows.yaml exists and skipped when it does not.
- `FlowStepForm.test.ts`: labels, help, widgets, hidden fields, `visible_when`,
`required_when` blocking submit, hidden and read-only fields not sent, and a
field outside `waiting.fields` shown read-only.
- Gates: Go tests of appbuild, flowcfg, flowengine, flowview, configedit, cli,
mcp and dataentry; frontend unit suite (6351 tests), typecheck and eslint;
golangci-lint on changed packages; arch-lint; comment-lint; `just docs`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
