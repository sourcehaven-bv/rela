---
id: IMPL-U5NAW4R
type: implementation-checklist
title: 'Implementation: Roll-ups along a chain, over ordered siblings and in list conditions'
started: "2026-11-28"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: storetest conformance and differential tests drive every backend; the example was validated with `rela validate`)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: edge cases are covered by tests)

**Verification Evidence:**
Go tests pass for the changed packages on the default, sqlite and postgres tags (pgstore suite 289 s against a local PostgreSQL). The omnifocus example validates and its roll-ups compute. golangci-lint is clean per package per tag; arch-lint, comment-lint and comment-body pass. The only failure, TestGoldenParity pile routes under `-tags postgres` in dataentry, fails the same way on origin/develop.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
