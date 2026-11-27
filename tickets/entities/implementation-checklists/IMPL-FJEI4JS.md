---
id: IMPL-FJEI4JS
type: implementation-checklist
title: 'Implementation: plimsoll housekeeping: drop two dead directives, fix two stale prose references, document the store/Config exceptions and the #1452 ratchet bumps'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new code; tag removal is behaviour-neutral)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: no behaviour change)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: none identified)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no runtime code)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests added)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests added)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests added)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests added)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests added)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: none identified)

**Verification Evidence:** `just plimsoll` passes. `go test
./internal/dataentryconfig/` passes. A throwaway program showed encoding/json
marshals and unmarshals an embedded struct identically with and without
`json:",inline"`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no runtime code)
- [x] No debug code left behind
