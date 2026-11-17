---
id: IMPL-2XJVN8T
type: implementation-checklist
title: 'Implementation: LLM actions: follow-ups (auto-apply, create outputs, batch runs)'
started: "2026-11-15"
completed: "2026-11-15"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: not run against a live AI provider; the HTTP route, the runner and the job handler are covered by tests with a fake provider and a fake queue)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: edge cases are covered by automated tests, see planning)

**Verification Evidence:** go test for dataentry, appbuild, dataentryconfig,
dataentrywire, llmtask, mcp, archguard, cmd/rela-server: pass. Frontend
typecheck, eslint (no errors) and entity component tests (1002): pass.
golangci-lint on changed packages: 0 issues. arch-lint, comment-lint, plimsoll:
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
