---
id: IMPL-P5ZA5A
type: implementation-checklist
title: 'Implementation: Writes require read access to the existing row'
started: "2026-10-12"
completed: "2026-10-12"
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

**Verification Evidence:** Each acceptance criterion has a test that compares
the answer for a hidden row with the answer for a missing one and checks the row
is unchanged: `TestWriteRequiresRead_*` (entity, soft delete, relation, copy
target, gate error, hidden source under a real policy),
`TestVersionTags_HiddenRowReadsAsMissing`, and the CalDAV tests in
caldav_write_requires_read_test.go (PUT and DELETE through the HTTP surface with
a Declarative policy shared by router and manager). The CalDAV PUT test fails
when the hidden check is removed. `go test ./...` passes; lint clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
