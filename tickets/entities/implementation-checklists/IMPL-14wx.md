---
id: IMPL-14wx
status: done
title: 'Implementation: Replace hardcoded ID comparisons in test assertions'
type: implementation-checklist
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: refactor of existing tests)
- [x] ~~Integration tests written~~ (N/A: test refactor, no new functionality)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place~~ (N/A: test refactor only)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: test refactor verified by test pass)
- [x] ~~Each acceptance criterion verified~~ (N/A: verified via test pass)
- [x] ~~Edge cases manually verified~~ (N/A: verified via test pass)

**Verification Evidence:**
All tests pass after refactor - verified via `just test`

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
