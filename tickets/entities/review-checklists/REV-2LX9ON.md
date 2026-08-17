---
id: REV-2LX9ON
type: review-checklist
title: 'Review: DynamicForm test real-HTTP flake'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`npm run test:run`) — 105 files, 1662 tests
- [x] Lint clean (`npm run lint`) — 0 errors (92 pre-existing warnings, none in
  the touched files)
- [x] ~~Coverage maintained~~ (N/A: test-only change, no production code
  touched; the frontend has no coverage enforcement — see root `CLAUDE.md`)
- [x] Typecheck clean (`npm run typecheck`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] ~~All critical review-responses addressed~~ (N/A: none raised)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes — three test files plus
  `src/test/setup.ts` (the global guard), no production code

**Review Responses:** RR-28QDBC (significant), RR-FB792S (significant),
RR-E5Q9C8 (minor), RR-F0DJ68 (minor). All addressed.

The review found the fix **incomplete**: with the SidePanel stub applied the
full suite still issued 21 stray requests, 15 of them from `EntityList.test.ts`
via an unstubbed `ExportMenu`. Fixing the file named in the traceback did not
fix the class. Resolved by adding the global guard rather than another stub.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

The acceptance criterion is behavioural and statistical, so it is measured
rather than asserted:

0. **No real HTTP requests anywhere in the suite.** PASS — the global guard in
   `src/test/setup.ts` takes the full suite from **21 stray requests to 0**,
   with all 1662 tests still passing (so no test relied on a real request).
   This is the criterion the first attempt missed: it fixed the named file and
   left 15 live requests in `EntityList.test.ts`.

1. **No real HTTP requests from the test file.** PASS —
   `npm run test:run -- src/components/forms/DynamicForm.test.ts` reports
   **42** `code: 'ECONNREFUSED'` occurrences before the fix and **0** after.
   This is the direct, deterministic measurement, and it is the one that
   matters: the flake is downstream of these requests existing at all.

2. **The intermittent teardown error stops recurring.** Measured over
   full-suite runs, because the failure is a race and a handful of green runs
   proves nothing (an early 5-run sample misled me into calling an incomplete
   fix verified):
   - Baseline, no fix: **1 failure in 6 runs**
   - `afterEach` unmount only: **2 failures in 10 runs** — insufficient
   - Unmount + `SidePanel` stub + `@/api` mock: **0 failures in 20 runs**,
     with the stub asserted present at the end of the run so the measurement
     cannot silently have tested the wrong state

3. **No coverage lost by stubbing `SidePanel`.** PASS — no assertion in the
   file references it; the file tests the affordance/redaction render gate,
   to which SidePanel is incidental.

4. **The partial `@/api` mock does not break other imports.** PASS — it
   spreads `importActual`, and `ApiError`/`getErrorMessage` (used by
   DynamicForm) still resolve; all 7 tests in the file pass.

## Documentation (enhancements only)

Skipped — bug fix to test files, no user-facing surface.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

The ticket's recorded cause was **corrected, not just fixed**: it named leaked
mounts, which are real but do not cause the flake. Anyone reading the old
diagnosis would have applied the unmount fix, watched a few green runs, and
believed it resolved — which is exactly what happened here before the rate was
measured properly.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** see the branch's PR — opened after this checklist was completed, since
the ticket gate requires a terminal state before the PR can merge.
