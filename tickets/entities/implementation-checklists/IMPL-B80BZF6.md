---
id: IMPL-B80BZF6
type: implementation-checklist
title: 'Implementation: rrule_next returns the after date itself for a rule without DTSTART'
started: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the Lua tests run `rela.rrule_next` through the Lua runtime; the Atlas recurrence script was run against both binaries (evidence below))
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: extends the existing table-driven Lua tests)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: assertions compare date strings only)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
`rela script` on 2026-10-30 with a binary from develop and one from this branch:

| call | develop | fix |
| --- | --- | --- |
| `rrule_next("FREQ=DAILY", today)` | 2026-10-30 | 2026-10-31 |
| `rrule_next("FREQ=YEARLY;BYMONTH=10;BYMONTHDAY=30", today)` | 2026-10-30 | 2027-10-30 |
| `rrule_next("FREQ=YEARLY;BYMONTH=10;BYMONTHDAY=1", "2025-10-01")` | 2027-10-01 | 2026-10-01 |
| `rrule_next("DTSTART:20241001T000000Z\nFREQ=YEARLY;...", "2025-10-01")` | 2026-10-01 | 2026-10-01 |

The Atlas `scripts/tests/test-recurrence.py` passes with the fixed binary.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
