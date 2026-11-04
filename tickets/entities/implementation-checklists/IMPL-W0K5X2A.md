---
id: IMPL-W0K5X2A
type: implementation-checklist
title: 'Implementation: Add a decimal property type'
started: "2026-11-03"
completed: "2026-11-03"
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

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Built `rela` and ran it on a scratch project with
`price: {type: decimal, scale: 2, min: 0, display: {currency: EUR}}`, `qty:
decimal` and a computed `total = entity.price * entity.qty`:
- `create -P price=12.50 -P qty=3` stored `price: "12.50"` (quoted, trailing
zero kept) and `total: "37.5"`; `9 * 0.1` stored `"0.9"` (exact).
- `price=10.125` saved with a `property_value_invalid` warning naming the
scale; `analyze properties` lists it.
- `list --filter "entity.price > 9.5"` returned 12.50 and 10.125, not 9;
`--where price=12.5` matched "12.50"; `--sort price` ordered 9, 10.125, 12.50.
- Storetest `DecimalText` passes on fs, memory, sqlite and postgres.
The SPA was verified by vitest unit tests only, not in a browser.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — `metamodel.IsNumericType` replaces
integer-or-decimal checks; `metamodel.ParseDecimal`/`DecimalText` are the one
parser on the Go side and `utils/decimal.ts` on the SPA side
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
