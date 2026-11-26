---
id: IMPL-J1XXN52
type: implementation-checklist
title: 'Implementation: Deep-linked list filter on a hyphenated relation name is dropped'
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the bug is in a pure URL parser; the deep link was checked by hand against the running server)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: an invalid name is dropped from the filter state by design; there is no error to surface)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: inputs are single query strings)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built the SPA and `rela-server`, ran it on `prototypes/data-entry/project` as
`alice@example.com`, and opened `/list/all_tickets?filter[belongs-to]=frontend`.
The list request is `GET /api/v1/tickets?...&filter[belongs-to]=frontend` and
the `belongs-to: frontend` chip shows. Before the fix the request had no filter.
The edge cases (prototype keys, quotes, control characters, outer whitespace)
are covered by the vitest cases, which fail on the old code.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: dropping an unusable URL key is the intended behaviour)
- [x] No debug code left behind
