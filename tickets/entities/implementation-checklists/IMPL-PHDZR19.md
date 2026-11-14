---
id: IMPL-PHDZR19
type: implementation-checklist
title: 'Implementation: Relation checks in enforce: write validation rules'
started: "2026-11-14"
completed: "2026-11-14"
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

CLI built from the branch, scratch project with the GRC rule
`behandelplan-nodig` (risico in_behandeling needs heeft_behandelplan):

- `update RIS-001 status=in_behandeling` without a plan: refused, message names
the rule only, exit 1.
- `link RIS-001 heeft_behandelplan BP-001`, then the same update: accepted.
- `unlink` of the only plan while in_behandeling: refused.
- `create risico status=in_behandeling` (RIS-002): accepted (create-then-link);
`analyze validations` lists it. A title edit on it is accepted (legacy row).
- Status to gesloten, then `unlink`: accepted (leaving the when-scope).

Automated: writerule unit tests, entitymanager tests on memstore and fsstore
(update, patch, legacy, delete, If-Match delete, replace, far-end rule,
create-then-link, hidden-plan ACL oracle), dataentry 422 tests for DELETE and
PATCH relations, storetest Tx case on mem, fs, sqlite and postgres.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
