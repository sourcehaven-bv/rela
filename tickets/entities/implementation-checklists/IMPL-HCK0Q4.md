---
id: IMPL-HCK0Q4
type: implementation-checklist
title: 'Implementation: OAuth token binding and Basecamp reference connector'
started: "2026-10-08"
completed: "2026-10-08"
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

**Verification Evidence:** basecamp_example_sqlite_test runs the shipped
examples/basecamp files against a Basecamp and Launchpad stub: paged pull with
ETag 304s, fixed point within two rounds, no push HTTP after a pull, push keeps
unowned fields, completion endpoints, create from rela, recorded conflict keeps
the base, 429 on pull and push, 401 invalidate with rotated token and one
retry, vanished todo. tokens_postgres_test: two pools, exactly one refresh
(three consecutive passes). consent.sh shellcheck clean. Builds and race tests
on default, sqlite and postgres tags; lint, arch-lint, comment-lint, plimsoll
clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
