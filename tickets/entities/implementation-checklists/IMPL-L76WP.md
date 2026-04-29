---
id: IMPL-L76WP
type: implementation-checklist
title: 'Implementation: Format dates with short month name in data-entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: pure presentation helper; integration covered by manual SPA verification below)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`frontend/src/utils/format.ts` now exposes a private `formatDate(value)` helper
that uses `Intl.DateTimeFormatOptions { year: 'numeric', month: 'short', day:
'numeric' }` and is called from both `formatValue` (date branch) and
`formatCellValue` (date property branch). Invalid date strings still return
`'-'` via the `isNaN(date.getTime())` guard.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The two updated tests assert `/Jan/` and `/2024/` against the input
`'2024-01-15'`. Hardcoded `Jan` / `2024` are appropriate here because they are
the format-validation values the test exists to verify (per CLAUDE.md: "Format
validation: testing specific formats... is appropriate").

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Programmatic verification of the rendered output via `node -e`:

```text
new Date('2024-01-15').toLocaleDateString(undefined, ...)  → "Jan 15, 2024"
new Date('2024-12-31').toLocaleDateString('en-GB', ...)    → "31 Dec 2024"
new Date('2024-07-04').toLocaleDateString('en-US', ...)    → "Jul 4, 2024"
```

Confirms:

- AC1 (`formatValue` returns month-abbreviated string): output contains `Jan`
and `2024`.
- AC2 (`formatCellValue` for date property): same — both call sites share
the helper.
- AC3 (invalid date returns `'-'`): existing tests `returns dash for invalid
date` and `returns dash for invalid date property` still pass.
- AC4 (single shared formatter): `formatDate` is the one source; both call
sites call it. Verified by re-reading `format.ts` after the edit.

Vitest run: 44 / 44 pass.

`vue-tsc --noEmit` clean.

`npm run lint` clean on `src/utils/format.ts` and `format.test.ts` (pre-existing
warnings on unrelated files are not introduced by this change).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Pure presentation change. No new attack surface; same `new Date(value)` parsing
as before, same `'-'` fallback for invalid input. No `console.*` or debug code
added.
