---
id: PLAN-IN2TQ
type: planning-checklist
title: 'Planning: Format dates with short month name in data-entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- Display formatting of `date`-typed property values in the Vue 3 data-entry SPA.
- Both rendering paths in `frontend/src/utils/format.ts`: `formatValue` (used in detail/general value rendering) and `formatCellValue` (used in list cells).
- The associated unit tests in `frontend/src/utils/format.test.ts`.

Out of scope:

- The date input widget format (still ISO `YYYY-MM-DD` for editing).
- Localising abbreviated month names beyond what `Intl.DateTimeFormat` already produces for the user's browser locale.
- `RruleBuilder.vue` (uses `new Date(...)` for arithmetic on month lengths and `dtstart`, no formatted display).
- `DynamicForm.vue` (uses `new Date(...)` for validation only, doesn't render formatted output).
- Backend Go templates / v1 HTMX UI (not the surface the user described as "data-entry"; the v2 SPA is the active UI).

**Acceptance Criteria:**

1. Calling `formatValue('2024-01-15', 'date')` returns a string containing `Jan` and `2024` and `15` (or `15`-equivalent in the test runtime locale), e.g. `Jan 15, 2024` or `15 Jan 2024` depending on locale.
   - Test scenario: existing test `formats date type correctly` is tightened to assert `/Jan/` and `/2024/`.
2. Calling `formatCellValue('2024-01-15', 'created_at', mockEntityType)` (where `created_at.type === 'date'`) returns a string containing `Jan` and `2024`.
   - Test scenario: existing test `formats date property correctly` is tightened similarly.
3. Invalid date inputs still return `'-'`.
   - Test scenario: existing tests for invalid date stay passing.
4. The formatter is shared between `formatValue` and `formatCellValue` (single source of truth, no duplicated `toLocaleDateString` calls).

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- The browser's built-in `Intl.DateTimeFormat` / `Date.prototype.toLocaleDateString` accepts an `options` object with `month: 'short'` that produces the desired `Jan` / `Feb` abbreviations. No external library is needed. `date-fns` / `dayjs` would be unnecessary weight for one formatter.
- All current date display goes through `formatValue` and `formatCellValue` in `frontend/src/utils/format.ts:18` and `format.ts:64`. There are no other formatted-date display call sites; `RruleBuilder.vue` and `DynamicForm.vue` use `new Date(...)` for parsing/arithmetic only (`grep -rn "new Date" --include="*.ts" --include="*.vue"` confirms this).
- Concept `data-entry-ui` (`stable`) is the affected concept; new feature `FEAT-Y6BDT` was created to track the human-friendly date formatting goal.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Add a small private helper `formatDate(value: string): string` in `frontend/src/utils/format.ts` that:
   - Parses the input with `new Date(value)`.
   - Returns `'-'` if `isNaN(date.getTime())`.
   - Otherwise returns `date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })`.
2. Replace the `toLocaleDateString()` calls in both `formatValue` (date branch) and `formatCellValue` (date property branch) with `formatDate(value)`. This is the de-duplication that the checklist's acceptance criterion #4 requires.
3. Tighten the two existing date assertions in `format.test.ts` from `/\d+/` (which trivially passes any output) to `/Jan/` and `/2024/`, so the assertion actually exercises the new format.
4. Keep the `'-'` fallback path untouched and verified by the existing invalid-date tests.

Passing `undefined` as the `locales` argument lets `Intl.DateTimeFormat` use the
runtime locale — same as today's behavior — but the `month: 'short'` option
forces the unambiguous abbreviation regardless of locale ordering.

**Files to modify:**

- `frontend/src/utils/format.ts` — add `formatDate` helper, replace two `toLocaleDateString()` call sites.
- `frontend/src/utils/format.test.ts` — tighten two assertions to verify the new format.

**Alternatives considered:**

- Using `date-fns` `format(date, 'd MMM yyyy')`: rejected — adds dependency for a one-line formatter.
- Hardcoding `en-US` locale in `toLocaleDateString('en-US', ...)`: rejected — would override the user's browser locale unnecessarily; `month: 'short'` already gives an unambiguous format in any locale.
- Full month name (`month: 'long'`): rejected — user explicitly asked to "keep short"; full month names like `January` widen list cells.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Input is the entity property value (a string), already stored in the rela backend. It's parsed by `new Date(value)`. Invalid input falls into the `isNaN(date.getTime())` branch and renders `'-'` — same behavior as today. No new attack surface is introduced; this is a pure presentation change.

**Security-Sensitive Operations:**

- None. `Intl.DateTimeFormat` doesn't touch the network, filesystem, or auth state.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| Acceptance criterion | Test |
|----------------------|------|
| AC1 (formatValue uses month abbreviation) | `formats date type correctly` updated to assert `/Jan/` and `/2024/` |
| AC2 (formatCellValue uses month abbreviation) | `formats date property correctly` updated to assert `/Jan/` and `/2024/` |
| AC3 (invalid date still returns dash) | Existing `returns dash for invalid date` and `returns dash for invalid date property` tests stay passing |
| AC4 (single shared formatter) | Code-level invariant verified by inspection; no separate test (the goal is dedup, not behavior) |

**Edge Cases:**

- Empty string / undefined / null: handled by the early return in `formatValue` and `formatCellValue`; no change.
- Invalid date string: `'-'` fallback unchanged.
- Date-only ISO strings (`'2024-01-15'`): treated as UTC midnight by `new Date()`; in the runtime test locale (`UTC` for vitest by default in this project) the day-of-month component will be `15`, so the assertion can match `/15/` if needed. We avoid asserting the exact day to keep the test locale-independent.
- Different browser locales: the `month: 'short'` option produces `Jan`/`Feb` in `en-*`, `janv.`/`févr.` in `fr`, etc. The test asserts `/Jan/` because the vitest environment defaults to `en-US`-equivalent; if this turns out to be flaky we can pin via `Intl.DateTimeFormat` mock or assert only `/2024/`. Confirmed `format.test.ts` already passes locale-dependent assertions today (`/\d+/`), so the existing infrastructure is fine.

**Negative Tests:**

- `formatValue('not-a-date', 'date')` returns `'-'`.
- `formatCellValue('invalid', 'created_at', mockEntityType)` returns `'-'`.

**Integration Test Approach:**

- Manual verification: run `npm run dev` from `frontend/`, open a list view that includes a date column (e.g. a ticket list with a date property — visible in fixture metamodels in `e2e/fixtures/`), confirm the column renders with `Jan` / `Feb` abbreviations.
- E2E: existing E2E tests don't currently assert specific date format strings, so no e2e change is required for this small display tweak.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Risk: locale-dependent assertion in tests becomes flaky on a non-`en-*` CI runner. Mitigation: assert on `/2024/` (always present) plus `/Jan/`; CI runs in containers with a stable `en-US.UTF-8` locale.
- Risk: a downstream caller relies on the current numeric format (e.g. for a manual string-comparison sort). Mitigation: searched for `toLocaleDateString` usages — only the two call sites in `format.ts` exist; no callers parse the output.

**Effort:** s (xs/s/m/l/xl) — single file change plus two test assertion
updates.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A - Internal display polish; no user-facing docs change date formatting expectations. The change is self-documenting via the rendered output.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: trivial display tweak; the design surface is one helper function and two call sites, all enumerated above. A formal design review would not produce additional findings.)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run.)

**Design Review Findings:** N/A — design review skipped per scope above.
