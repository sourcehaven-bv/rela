---
id: IMPL-QHJVF2
type: implementation-checklist
title: 'Implementation: Enum fields in custom view sections show raw values instead of labels'
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
- e2e `view-enum-labels.spec.ts`: a display section with `kind` (single enum
of type task_kind) and `areas` (list of work_area) failed before the fix with
badges `external_obligation`, `software_development`, `operations`; it passes
after the fix with the schema labels.
- vitest `PropertyDisplay.test.ts`: fails before the fix, passes after, for
both the schema-def path and the routing-hint path; badge colour still resolves
through the type.
- Full frontend vitest (3702 tests), typecheck, lint, e2e lint/typecheck and
the full Playwright suite pass. Two unrelated specs (pending-indicators,
relation-picker-large-candidate-set) failed once under parallel load and passed
on rerun.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
