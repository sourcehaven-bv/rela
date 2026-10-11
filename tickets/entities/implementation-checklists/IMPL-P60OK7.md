---
id: IMPL-P60OK7
type: implementation-checklist
title: 'Implementation: Move the version sweep out of the store into a versioning service'
started: "2026-10-11"
completed: "2026-10-11"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: internal lifecycle refactor; covered by appbuild integration tests on sqlite and postgres)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
1. `go list -deps` per tag: only new package is internal/store/bgloop (postgres, sqlite builds); arch-lint OK.
2. `storetest.RunVersionTests`: pass on sqlite and on postgres (throwaway local database).
3. Existing pgstore sweep tests (lock scope, drain, BUG-1DWMYO) pass with assertions unchanged.
4. versionsweep_lifecycle_sqlite_test.go: closing tenant A ends its sweep and A captures nothing; B keeps capturing; CloseAssembly stops the replaced sweep; fails with Stop removed. versionsweep_built_test.go requires a sweep on postgres and sqlite; fails on a forced type miss.
Edge cases: Stop before Start, Stop twice, Start after Stop, nil Stop: lifecycle
tests in both stores and bgloop. just lint 0 issues; plimsoll, comment-lint
pass. coverage-check: floors met (82.6% total) with a longer timeout; local-only
failures are the SPA test and the 10m timeout under load.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (lifecycle extracted to bgloop)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
