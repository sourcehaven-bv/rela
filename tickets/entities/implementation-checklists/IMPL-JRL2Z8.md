---
id: IMPL-JRL2Z8
type: implementation-checklist
title: 'Implementation: Form save drops all relation edits when the entity has a relation the form does not render'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Edge cases covered: `link_as: to` prefill of an unrendered relation (kept via
`alsoKeep`, so the submit-site post-condition check still passes);
affordance-hidden relation fields (ownership reads `allFields`, not `fields`); a
form rendering both directions of one relation (outgoing half stays owned
regardless of field order); empty relation lists, which legitimately mean "clear
all edges" and must not be reported as untyped.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no
interpolated values in these assertions)
- [x] Property comparisons use original object, not hardcoded strings

Fixtures mirror the reported shape (a `taak` with `gaat_over` rendered outgoing
and `onderdeel_van` unrendered) through a `mountEdit` helper, matching the
existing `DynamicForm.test.ts` harness.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Driven in Chrome via Puppeteer against the atlas project on the postgres
backend, reproducing the reported steps on `/form/edit_taak/TASK-7F8K`:

- Before: error toast "Some related entities have unknown types; relation
changes were not saved.", no PATCH sent.
- After: no toast, chips 11 -> 12, and the captured PATCH body carried
`gaat_over` with 12 typed resource identifiers and no `onderdeel_van`.
- Server re-read confirmed `PROCEDURE-00R4` persisted and
`onderdeel_van: [PROJ-DP5C]` preserved (an absent relation key means "leave
alone", so dropping unowned keys is non-destructive).
- Entity restored to its original 11 links afterwards.

Mutation check: reverting `ownedRelationKeys` to the pre-fix pass-through fails
both integration tests, so they pin the fix rather than passing vacuously.

Suite: 196 files / 3177 tests pass; `npm run typecheck` clean; `npm run lint` 0
errors.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The ownership decision is a pure module (`ownedRelations.ts`) beside
`prefillRouting.ts`, following that file's precedent of keeping the subtle
decision testable without a form mount. Both save paths call the same helper
rather than repeating the filter. Narrower payloads only; no new capability, and
the read gate is untouched.
