---
id: REV-5G29M
type: review-checklist
title: 'Review: Fix rrule property display in lists to match detail page'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `npm run test:run` from `frontend/`: 554/554 passed
- [x] Lint clean — `npm run lint`: 0 errors in `format.ts`/`format.test.ts` (pre-existing warnings in unrelated files only)
- [x] ~~Coverage maintained~~ (N/A: frontend uses per-file 100% ratchet via separate workflow; new tests increase coverage of `format.ts`)

Note: backend coverage check (`just coverage-check`) is irrelevant for a
frontend-only change. `npm run typecheck` also clean.

## Code Review

- [x] Run cranky-code-reviewer agent (equivalent to `/code-review`)
- [x] All critical review-responses addressed (none filed)
- [x] All significant review-responses addressed (RR-HJF2P, RR-PZPIK, RR-70BG2 — all `addressed`)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

- RR-HJF2P (significant) — array short-circuit swallowing array-shaped rrule values → addressed: reordered branches; rrule unwraps single-element array; regression test added
- RR-PZPIK (significant) — null/empty asymmetry undocumented → addressed: comment added at top of formatCellValue
- RR-70BG2 (significant) — tests asserted parity against normalized input → addressed: refactored to `it.each` table asserting `formatCellValue(input,...) === formatValue(input,'rrule')`
- RR-89CVV (nit) — redundant `String(value)` → addressed: removed
- RR-3S4DO (nit) — duplicated test literals → addressed: eliminated by `it.each` refactor
- RR-NGQPK (minor) — broader architectural unification → deferred (out of scope for xs ticket; documented as follow-up)

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. RRULE in list column renders human-readable text identical to detail page → **PASS** (it.each parity test for bare/RRULE:/DTSTART forms)
2. Empty/null rrule renders as existing empty-cell representation → **PASS** (explicit null + empty string tests)
3. Malformed rrule falls back to raw string → **PASS** (parity test for `'NOT_A_RULE'`)
4. Unit tests cover the variants → **PASS** (8 rrule cases including new array-shape regression test)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug-style fix bringing list rendering in line with already-documented detail-page behaviour. No user-facing docs change.)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (Deferred: user invoked `/ticket` only; commit and PR creation are user-controlled steps to be invoked separately.)
- [x] ~~All CI checks pass~~ (Deferred: depends on PR creation above.)
- [x] ~~PR URL documented below~~ (Deferred: see above.)

**PR:** TBD — run `/pr` when ready to create.
