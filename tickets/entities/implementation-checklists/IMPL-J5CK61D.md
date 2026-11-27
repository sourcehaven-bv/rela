---
id: IMPL-J5CK61D
type: implementation-checklist
title: 'Implementation: Relation filter controls match on target id instead of display title'
started: "2026-11-25"
completed: "2026-11-25"
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

- Go: `go test ./internal/dataentry/...` passes. The new tests cover filtering by id
(eq and ne), two targets with the same title, an id filter after a rename, an
unreadable id treated as a title, and the pushdown compared with the Go path.
The postgres differential test passes on a local postgres.
- A mutation check: dropping `Negate` makes the differential test fail.
- SPA: the EntityTargetSelect and FilterBar vitest suites pass (32 tests).
- Manual: rela-server on a copy of the data-entry prototype, with a second
category titled "Backend" and a `belongs-to` filter control. The control lists
both "Backend" targets separately. Picking one sets `filter[belongs-to]=devops`
and shows only TKT-003. Via the API, the id `backend` returns TKT-001/002/005
and the title `Backend` returns both sets, and logs the deprecation warning
without the value.
- The e2e list spec is updated. It was not run locally, because the Playwright
browser is not installed. CI runs it.
- Found during manual testing, pre-existing and out of scope: the SPA drops a
URL filter whose relation name has a hyphen (`PROPERTY_NAME_RE` in
`frontend/src/utils/filters.ts`), so a `belongs-to` filter is lost on a direct
page load.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
