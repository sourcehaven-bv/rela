---
id: REV-ZKJNH
type: review-checklist
title: 'Review: Format dates with short month name in data-entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`npm run test:run` from `frontend/`: 552 / 552 pass (49 in `format.test.ts`).
Also re-ran under `TZ=America/Los_Angeles` and `TZ=Pacific/Pago_Pago` (UTC-11)
to confirm the timezone fix — 49 / 49 pass in both timezones.

`vue-tsc --noEmit`: clean.

`npm run lint`: 0 errors. Pre-existing warnings on unrelated files
(`stress/...`, etc.) are not introduced by this change; the changed files
(`format.ts`, `format.test.ts`) have no warnings.

`just coverage-check`: PASS (total 74.2%, all package floors satisfied).

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

| ID | Severity | Status | Summary |
|----|----------|--------|---------|
| RR-TUOOT | critical | addressed | Off-by-one timezone bug — added `parseDate()` that detects `YYYY-MM-DD` and constructs in local time, with overflow rejection. Verified under TZ=America/Los_Angeles and TZ=Pacific/Pago_Pago. |
| RR-H3K1D | critical | addressed | Locale-flaky test — added optional `locale` parameter; tests now pass `'en-US'` / `'en-GB'` and assert `toBe('Jan 15, 2024')` exactly. |
| RR-CLPQB | significant | addressed | Day-of-month now asserted (`/15/`) plus exact-string assertions in dedicated `formatDate` tests. |
| RR-ROE1T | significant | addressed | Confirmed `static/v2/` is gitignored; CI rebuilds via `just build` → `npm run build`. Local rebuild produced bundle with new format options. |
| RR-E32PY | significant | addressed | `formatDate` and `DATE_FORMAT_OPTIONS` now exported. |
| RR-PSJY9 | significant | addressed | `formatDate` returns `string \\| null`; callers substitute `'-'` (formatValue) / `''` (formatCellValue) per their own contracts. |
| RR-3K96E | minor | addressed | Added contract tests for empty / invalid / overflow input. |
| RR-7IVPA | minor | deferred | Premature optimization (cached `Intl.DateTimeFormat`); no measurement, easy 1-line swap later. |
| RR-H5NA5 | nit | deferred | User-configurable date format — out of scope per reviewer's own framing. |
| RR-9ZQLP | nit | deferred | `formatDateTime` companion — no `datetime` property type in metamodel yet. |

Self-review of the diff: `format.ts` adds `DATE_FORMAT_OPTIONS`, `DATE_ONLY_RE`,
`parseDate`, `formatDate`, and updates two call sites. `format.test.ts` adds an
import for `formatDate`, tightens two assertions, removes the obsolete `'-'`
invalid-cell test, and adds a `formatDate` describe block. No unrelated changes.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| AC1 (`formatValue` returns month-abbreviated string) | PASS | Test `formats date type with short month name and exact day` asserts `/15/` and `/2024/`; `formatDate` test asserts `toBe('Jan 15, 2024')` for `en-US`. |
| AC2 (`formatCellValue` for date property) | PASS | Test `formats date property with short month name and exact day` asserts `/15/` and `/2024/`. |
| AC3 (invalid date returns dash from formatValue, empty from formatCellValue) | PASS | Tests `returns dash for invalid date` and `returns empty string for invalid date property (matches cell-empty sentinel)`. |
| AC4 (single shared formatter) | PASS | Both call sites delegate to `formatDate`; verified by code inspection. |

Plus emergent acceptance criteria from review fixes:

| AC | Status | Evidence |
|----|--------|----------|
| Day-of-month preserved across timezones | PASS | Full suite passes under `TZ=America/Los_Angeles` (UTC-8) and `TZ=Pacific/Pago_Pago` (UTC-11). |
| Overflow input rejected | PASS | `formatDate('2024-13-45')` returns `null`. |
| Locale-deterministic output | PASS | `formatDate('2024-01-15', 'en-US')` returns exactly `'Jan 15, 2024'`. |

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: pure display-format polish; no user-facing doc references the prior numeric format. The change is self-evident in the rendered UI.)
- [x] ~~User-facing documentation updated~~ (N/A as above.)
- [x] ~~Docs-checklist marked as done~~ (N/A as above.)

**Docs Checklist:** N/A.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

`formatDate` is exported with a clear contract (string | null, optional locale)
and a comment documenting the timezone-safety rationale of `parseDate`.

## Pull Request

- [ ] ~~Run `/pr` command to create PR and monitor CI~~ (skipped at user direction; commit will be made on `develop` directly)
- [ ] ~~All CI checks pass~~ (will be verified post-push)
- [ ] ~~PR URL documented below~~

**PR:** N/A — direct commit per user workflow.
