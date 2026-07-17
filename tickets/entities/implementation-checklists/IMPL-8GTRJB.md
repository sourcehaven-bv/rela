---
id: IMPL-8GTRJB
type: implementation-checklist
title: 'Implementation: Add a datetime metamodel property type (time-bearing, with date+time form widget)'
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

- [x] Using fixture builders or factories for test data (RandomDatetime; table-driven subtests)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (backend via CLI + running rela-server)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Backend E2E (running binary against a scratch project with a `datetime`
property):
- **Create + store**: `rela create event -P starts_at="2026-07-13T14:30:00Z"` stores `starts_at: "2026-07-13T14:30:00Z"` (quoted UTC RFC3339). ✓ (AC1, AC2)
- **Filter `>`**: `--where "starts_at>2026-07-13T12:00:00Z"` returns only the 14:30 event, not the 09:00 one. ✓ (AC3)
- **Strict-instant `=`**: `--where "starts_at=2026-07-13"` (bare date = midnight) matches neither 09:00 nor 14:30 — documented behavior. ✓ (AC3 / RR-5R3QFJ)
- **Hand-edited UNQUOTED datetime** (`starts_at: 2026-07-13T20:00:00Z`, yaml→time.Time): `analyze properties` = "All valid" — the RR-NY7PRB critical fix confirmed live (a string-only arm would have rejected it). ✓ (AC2 / RR-NY7PRB)
- **Bare date** (`starts_at: 2026-07-13`): validates clean, sorts as midnight. ✓ (AC2 / RR-MYC2B6)
- **Sort chronological incl. time**: `--sort starts_at` → BareDate(00:00), Retro(09:00), Standup(14:30), Unquoted(20:00). ✓ (AC4)
- **Served type**: `GET /api/v1/entity-types` exposes `datetime` to the SPA. ✓ (AC6)

Frontend (unit-test matrix against the real `@date-fns/tz` + `Intl`, all
passing):
- UTC→effective-zone input round-trip (UTC zone: 12:30→`2026-07-13T12:30`; NY: →`08:30`). ✓ (AC7)
- Emit canonical `...Z` on user input (NY 09:30 → `2026-07-13T13:30:00Z`). ✓ (AC7)
- **Non-destructive**: mounting emits nothing; cleared input → `''`. ✓ (AC7 / RR-N1Z9BF)
- Non-integer offset `Asia/Kolkata` (+05:30 → 18:00) and date-line `Pacific/Auckland` (18:00Z → next-day 06:00). ✓ (AC10)
- Display mode formats in the effective zone. ✓ (AC8)
- Read-only tz indicator shows the effective zone. ✓ (RR-3A0RUL)
- uiStore tz pref persists/reads from localStorage; unsupported/stale zone falls back to browser. ✓ (AC9 / RR-3A0RUL)
- FieldRenderer routes a `datetime` property to the datetime-local widget end-to-end. ✓ (AC6)
- OpenAPI schema arm asserted `{type: string, format: date-time}`. ✓ (AC5)

**Not runnable here:** interactive Chrome click-through (browser extension not
connected). Covered instead by the widget unit-test matrix above + confirmed the
production bundle builds and the server serves the datetime type.

**Automated gates:**
- `go test ./...` — all pass (incl. new metamodel/filter/dataentry tests + a bonus `matchDate` time.Time fix).
- `just arch-lint` — no warnings.
- `npm run test:run` — 1263 pass (78 files). `npm run typecheck` — clean. `npm run lint` — 0 errors (pre-existing warnings only; no new). `npm run build` — SPA + editor bundles compile with `@date-fns/tz`.
- `npm audit` for `@date-fns/tz` — 0 vulnerabilities; relies on `Intl` (no bundled tzdata).

## Quality

- [x] Code follows project patterns (datetime mirrors the `date` type/widget throughout; uiStore tz pref mirrors the `theme` pattern)
- [x] Checked for DRY opportunities (shared `matchDate`/`compareDates`/`toTime`, `isTimeLike` helper, tz conversion helpers in `format.ts`; no premature abstraction)
- [x] No security issues introduced (allowlist validation; tz pref client-only + validated; no new I/O or trust boundary)
- [x] No silent failures (validation errors surfaced via the standard path; tz fallbacks are deliberate + documented)
- [x] No debug code left behind
