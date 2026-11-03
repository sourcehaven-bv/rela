---
id: IMPL-G4JTM8J
type: implementation-checklist
title: 'Implementation: SPA edit form saves an edit onto a different entity after in-form navigation'
started: "2026-10-30"
completed: "2026-10-30"
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

- `e2e/tests/form-entity-switch.spec.ts` drives the real flow: edit bug A's
description, follow the side-panel link to bug B's edit form. On `develop` it
fails (B's form shows A's text, A's edit never saved); with the fix it passes.
- Unit: `DynamicForm.entitySwitch.test.ts` (5), `useAutoSave.conflicts.test.ts`
target identity (5, incl. faced rows and a 412 refetch naming another entity),
`EntityDetail.world.test.ts` toggle, body and title across a row switch. Each
guard was mutation-checked.
- Full frontend suite: 355 files, 5397 tests pass; typecheck clean; eslint 0
errors.
- Related e2e specs (forms, entity-detail, checkboxes, conflicts,
faces-worlds, task-item-conversions, form-entity-switch): 48 pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
