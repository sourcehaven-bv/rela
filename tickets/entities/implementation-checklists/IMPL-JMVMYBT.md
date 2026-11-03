---
id: IMPL-JMVMYBT
type: implementation-checklist
title: 'Implementation: Mentions follow a rename: tombstones on database stores, in-place rewrite on fs'
started: "2026-11-02"
completed: "2026-11-02"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: verified by automated tests only; not exercised in a running app)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: edge cases covered by automated tests, not by hand)

**Verification Evidence:** Go tests pass on the default, memorybackend, sqlite
and postgres builds (storetest RunRetiredIDTests and the mention suite on every
backend; visibility, dataentry, archguard, sqlitedb). Frontend vitest (2492) and
vue-tsc pass. golangci-lint, arch-lint, plimsoll and comment-lint pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
