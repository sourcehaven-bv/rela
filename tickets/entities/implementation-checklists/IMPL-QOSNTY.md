---
id: IMPL-QOSNTY
type: implementation-checklist
title: 'Implementation: Corpus round-trip test exceeds its 180 s timeout'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: the change is to a test)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the change is to a test)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: test reads the repository corpus)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no new assertions)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no new assertions)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no new assertions)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no new assertions)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: test-only change; full sweep run locally)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Full sweep (RELA_FULL_CORPUS=1): 59 tests pass, 6,835
files, 0 semantic drift, 0 non-idempotent; 55 s locally, slowest chunk 13 s.
Sampled run: 38 tests pass. eslint, prettier and vue-tsc clean. No Go changes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
