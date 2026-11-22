---
id: IMPL-SNBW332
type: implementation-checklist
title: 'Implementation: Remove dead code and duplicated helpers'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Happy path implemented~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Edge cases from planning handled~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Only specifying values that matter for the test~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Each acceptance criterion verified with test scenario from planning~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Edge cases manually verified~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [x] ~~Code follows project patterns (check similar code)~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~Checked for DRY opportunities — repeated literals, expressions, or~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] ~~No security issues introduced~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
- [x] ~~No debug code left behind~~ (N/A: behaviour-preserving cleanup, no new input or user-facing change)
