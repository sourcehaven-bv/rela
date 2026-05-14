---
id: IMPL-STH7
type: implementation-checklist
title: 'Implementation: Extract automation.Runner with consumer-side Host interface'
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: pure refactor with no user-visible behavior change; verified by 14 pre-existing cascade tests passing unchanged)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: pure refactor; edge cases covered by 10 new Runner unit tests with stub Host — depth limit, if_exists handling, action order, error continuation, Lua error patching)

**Verification Evidence:**

- 9 non-Lua cascade tests in `internal/workspace/workspace_test.go` pass unchanged.
- 5 `TestLuaAutomation_*` integration tests pass against the real `script.Engine`.
- 10 new `autocascade.Runner` unit tests pass (`internal/autocascade/runner_test.go`).
- `internal/autocascade` package coverage: 71.2% (uncovered: `IfExistsReplace` branch).
- `just ci` green: lint, arch-lint, race-enabled tests, coverage, docs.
- See PLAN-V6UR "Acceptance Criteria" — all 7 met.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
