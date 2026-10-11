---
id: IMPL-414R5Q
type: implementation-checklist
title: 'Implementation: gate MCP counts through the read ACL'
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by the memstore + Declarative tests; no UI)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Test evidence:** `go test ./internal/...` green; `-race` on the new tests
green; golangci-lint, arch-lint, comment-lint, plimsoll clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — scope composition shared by list and count
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
