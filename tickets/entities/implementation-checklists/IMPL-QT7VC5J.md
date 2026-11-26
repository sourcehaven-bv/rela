---
id: IMPL-QT7VC5J
type: implementation-checklist
title: 'Implementation: TestServer_AwarenessExpiry_GoroutineStoppedOnEvict flakes under -shuffle'
started: "2026-11-26"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: test-only change)
- [x] ~~Happy path implemented~~ (N/A: test-only change)
- [x] ~~Edge cases from planning handled~~ (N/A: test-only change)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: one NewServer per test)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: test-only change)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: test-only change)

**Verification Evidence:**
`go test -race -shuffle=on -count=200 -timeout 3h ./third_party/ygows` passed (4766 s). Removing the `Destroy` call in `CloseRoom` or `Shutdown` makes the matching test fail with `expiry goroutine leaked: 1 still running`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
