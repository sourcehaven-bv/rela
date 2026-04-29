---
id: PLAN-RI4XB
type: planning-checklist
title: 'Planning: Fix rrule property display in lists to match detail page'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- Make `formatCellValue` (the list/table formatter in `frontend/src/utils/format.ts`) produce the same human-readable RRULE output as `formatValue` does for the detail page.
- Add unit tests covering the new branch.

Out of scope:

- Changing the RRULE editor / `RruleBuilder.vue`.
- Changes to detail-page rendering (already correct).
- Backend / metamodel changes.
- Any other property-type formatting drift between `formatValue` and `formatCellValue` (a separate issue if any).

**Acceptance Criteria:**

1. List columns with an rrule property render the same human text as the detail page.
   - Test: unit test `formatCellValue("FREQ=DAILY", "schedule", entityType)` returns the same string as `formatValue("FREQ=DAILY", "rrule")` (`"every day"`).
2. `RRULE:`-prefixed and `DTSTART:... RRULE:...` forms are accepted.
   - Test: unit tests assert both forms format to the same human text as the bare form.
3. Empty/null rrule renders as the existing empty-cell representation (currently empty string from `formatCellValue`, no regression).
   - Test: unit test asserts `formatCellValue(null, "schedule", entityType)` and `formatCellValue("", "schedule", entityType)` return `''`.
4. Malformed rrule strings fall back to the raw string (parity with `formatValue`).
   - Test: unit test asserts a malformed value is returned as-is (not "-" and not a thrown error).

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- The `rrule` npm package (already a project dep) exposes `RRule.fromString(...).toText()`. Already used by `formatValue` in `frontend/src/utils/format.ts:25-36`.
- The duplication between `formatValue` and `formatCellValue` is the root cause. The right pattern is to delegate `formatCellValue`'s typed-property branch to `formatValue` so both call sites stay in sync going forward.
- Related feature: FEAT-DUW9 (RRULE field type with data-entry UI widget). Related concepts: `data-entry-ui`, `views`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

In `formatCellValue`, when a `propDef` is found and its `type` is `rrule`,
delegate to `formatValue(value, 'rrule')`. Place the rrule branch alongside the
existing `date` and `boolean` branches.

Note that `formatValue` returns `'-'` for null/empty arrays/empty values, while
`formatCellValue` returns `''`. Keep `formatCellValue`'s existing null/empty
handling at the top of the function (already in place at line 53) — only
delegate when the value is a non-empty rrule string.

Alternatives considered:

- Inline the rrule formatting in `formatCellValue`: rejected — duplicates the parsing logic and the `RRULE:`/`DTSTART:` normalization, will drift again.
- Refactor both functions into one: out of scope. Their null/empty behaviour differs intentionally (list cells stay empty; detail page shows `-`). A targeted delegation is the smallest change.

**Files to modify:**

- `frontend/src/utils/format.ts` — add rrule branch to `formatCellValue`.
- `frontend/src/utils/format.test.ts` — add unit tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Input is the rrule string stored on the entity (user input via the RRULE form widget). It is parsed by `RRule.fromString` which throws on malformed input; existing try/catch in `formatValue` falls back to the raw string. Output is rendered as plain text (Vue interpolation, no v-html), so no XSS risk.

**Security-Sensitive Operations:**

- None. Pure formatting, no I/O, no auth.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1: `formatCellValue('FREQ=DAILY', 'schedule', entityTypeWithRruleProp)` → `'every day'`.
- AC2a: `formatCellValue('RRULE:FREQ=DAILY', ...)` → `'every day'`.
- AC2b: `formatCellValue('DTSTART:20240101T000000Z\nRRULE:FREQ=DAILY', ...)` → `'every day'`.
- AC3a: `formatCellValue(null, 'schedule', entityType)` → `''`.
- AC3b: `formatCellValue('', 'schedule', entityType)` → `''`.
- AC4: `formatCellValue('NOT_A_RULE', 'schedule', entityType)` → `'NOT_A_RULE'`.

**Integration / manual verification:**

- Find or create a project entity type with an rrule property and a list view that includes the rrule column. Verify in the running data-entry app that the cell renders the human-readable form. (This project's tickets/* metamodel doesn't have an rrule property; will use the existing `RruleBuilder.test.ts` fixtures or a temporary metamodel addition for manual smoke if available — otherwise unit-test parity is the primary verification.)

**Edge Cases:**

- Empty string vs null vs whitespace: handled by existing top-of-function null/empty guard plus the new branch's `if (typeof value === 'string' && value)` guard.
- Array-typed rrule (not expected in metamodel but handled defensively): existing `Array.isArray` branch fires before the rrule branch — unchanged behaviour.
- Property without `propDef` (unknown column): falls through to `String(value)` — unchanged.

**Negative Tests:**

- Malformed rrule → falls back to raw string (covered by AC4).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Low. Pure-function change with unit test coverage. The only behaviour change for non-rrule properties is none (the new branch is gated on `propDef.type === 'rrule'`).
- Effort: xs.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — bug-style fix bringing list rendering in line with already-documented detail-page behaviour. No user docs change. Existing inline comment on the rrule branch in `formatValue` is sufficient.

## Design Review

- [x] Skipped — change is a one-line delegation to an existing, already-reviewed code path. Risk is bounded to a single pure function with full unit-test coverage.

**Design Review Findings:** none
