---
id: IMPL-S3XSQ0
type: implementation-checklist
title: 'Implementation: Create a linked entity from a markdown task item'
started: "2026-10-18"
completed: "2026-10-20"
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

- Go: `go test ./internal/...` passes after the review fixes, rebased on
develop `b4e53fd22`. `just coverage-check` passes (84.7% total). New handler
tests cover each create error class, the kept entity, the follow-up warning and
the body limit; new CreateLinked tests cover BeforeWrite, full bounds in both
directions and two ends. Disabling the pre-write edge check fails
`TestCreateLinked_FullBoundWritesNothing`.
- Goldens: regenerated. The only change is the new `warnings` field in the
OpenAPI documents.
- Frontend: `npm run test:run` passes 321 files and 4559 tests; `vue-tsc`,
ESLint and Prettier are clean on the changed files.
- E2E: `task-item-conversions.spec.ts`, `checkboxes.spec.ts` and
`pending-indicators.spec.ts` pass (16 tests).
- Manual, scripted with Playwright against a scratch copy of the prototype
project with two recipes on `ticket`. The link recipe created TKT-007 and
replaced the item with a reference. The remove recipe, with a required field
left empty, opened the fields dialog, created the ticket after input, removed
the item and lifted its nested item. Checked in light, dark and touch (390px):
the button shows on hover, and always on touch. No page errors.
- Gates: `just lint`, `arch-lint`, `plimsoll` and `comment-lint` are clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
