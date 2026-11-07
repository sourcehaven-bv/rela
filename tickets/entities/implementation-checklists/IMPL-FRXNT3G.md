---
id: IMPL-FRXNT3G
type: implementation-checklist
title: 'Implementation: Two-tab autosave property test misses its out-of-attempts floor at random'
started: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: the change is to a test; it is verified by running that test over 200 seeds)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: test-only change)
- [x] Happy path implemented (exhaust binds to one save; floor run is seeded and names its seed)
- [x] Edge cases from planning handled (an exhaust armed while another is running waits; a tab's two channels count as two saves)
- [x] Error handling in place (a floor failure names PROP_SEED)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: generated commands, unchanged)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (200-seed loop, old and new code)
- [x] Each acceptance criterion verified with test scenario from planning (no seed under any floor; PROP_SEED=8, under the floor on the old code, passes)
- [x] Edge cases manually verified (debug log: every armed exhaust got its three 412s within one save)

**Verification Evidence:**
Seeds 1 to 200, 300 runs each, `exhausted` total against the floor of 30:

| Code | Min | Mean | Seeds under 30 |
|---|---|---|---|
| old | 23 | 39.8 | 7 (8, 17, 39, 117, 130, 162, 170) |
| new | 46 | 64.5 | 0 |

No seed had a property violation on the new code. The other floors' minimums on the new code: accepted 847 (floor 600), 412s 328 (300), reported 254 (150), merged 51 (37.5), faults 498 (300).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (the two empty-412 answers share `emptyPrecondition`) — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
