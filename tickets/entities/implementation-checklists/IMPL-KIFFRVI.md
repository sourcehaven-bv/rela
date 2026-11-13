---
id: IMPL-KIFFRVI
type: implementation-checklist
title: 'Implementation: rela test v1 (3/4): rela test command, rela.test Lua module, seeded fakers and principals'
started: "2026-11-13"
completed: "2026-11-13"
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

- `internal/apptest/apptest_test.go` has one test per acceptance criterion
(AC1-9, AC11-13, AC15, AC16 with goleak over 100 cases). All pass with `-race`;
package coverage is 77.5%.
- `internal/cli/test_test.go` checks the exit codes, no tests and missing
config. `internal/cli/test_sqlite_test.go` (AC14) runs `rela test` while another
process holds the rela.db lock.
- Manual run of the built `rela` on a copy of the testdata project with three
cases: the passing case reported `ok`, the failing case reported
`tests/demo.test.lua:13: expected 2, got 1`, the run printed the seed and exited
1. `--run viewer` ran one case and exited 0. `-o json` printed the report. A
hash of the project tree was the same before and after.
- A done-mail to a recipient outside `also_allow` failed the case with
`recipients_not_allowed`, so RecipientPolicy still applies.
- Gates: `just arch-lint`, golangci-lint on the changed packages,
`just comment-lint` and `just plimsoll` are clean. The postgres `rela` and
`rela-server` link no bleve.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
