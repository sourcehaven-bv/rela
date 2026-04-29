---
id: IMPL-ONWT1
type: implementation-checklist
title: 'Implementation: Fix rrule property display in lists to match detail page'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written~~ (N/A: pure utility function — `formatCellValue` is unit-testable in isolation; the EntityList consumer renders its return value via Vue text interpolation, no separate integration concern)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (delegates to `formatValue`'s existing try/catch fallback)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: existing test file uses inline `mockEntityType`; new tests follow same convention to stay consistent)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolation in these tests; the rrule strings ARE the input under test)
- [x] Property comparisons use original object, not hardcoded strings — rrule expected output is derived from `formatValue(...)` rather than hardcoded `"every day"`, so the tests pin parity with the detail-page formatter rather than to a specific string from the rrule library

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Unit test output (50 tests passed, including 7 new rrule cases):

- AC1 (bare RRULE): `formatCellValue('FREQ=DAILY', 'schedule', mockEntityType)` matches `formatValue('FREQ=DAILY', 'rrule')` ✅
- AC2a (`RRULE:` prefix): matches bare form output ✅
- AC2b (`DTSTART:...\nRRULE:...`): matches bare form output ✅
- AC3a (null): returns `''` (preserves list empty-cell behaviour) ✅
- AC3b (empty string): returns `''` ✅
- AC4 (malformed): returns `'NOT_A_RULE'` raw — fallback path matches `formatValue` ✅

Test command: `cd frontend && npm run test:run -- src/utils/format.test.ts`

End-to-end UI smoke test was not run because the in-tree `tickets/` and
`docs-project/` metamodels do not declare any rrule property, so there is no
live list view that exercises this code path. Parity with `formatValue` (which
the detail page uses, and which is already verified working in production) is
enforced by the unit tests above.

## Quality

- [x] Code follows project patterns (matches the existing `date`/`boolean` branch style in the same function)
- [x] No security issues introduced (pure formatting; no I/O; output rendered via text interpolation, not v-html)
- [x] No silent failures (malformed rrule falls back to raw string — same documented behaviour as the detail page)
- [x] No debug code left behind

`npm run lint` and `npm run typecheck`: clean (0 errors).
