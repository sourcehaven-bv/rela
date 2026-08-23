---
id: REV-IPWS2P
type: review-checklist
title: 'Review: Framework-level loading/pending indicator system for the data-entry SPA'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — Go suite clean, no FAIL
- [x] Lint clean (`just lint`) — golangci-lint **0 issues**; `just arch-lint` OK; frontend eslint **0 errors** (57 pre-existing warnings)
- [x] Comment lint gate clean (`just comment-lint`) — **no findings across 10183 comments**
- [x] Coverage maintained — Go floors unaffected (no Go changes); the frontend has no coverage enforcement by design (root CLAUDE.md)

Frontend: **1866 unit tests / 117 files pass**, `vue-tsc` clean. E2E: **265
passed, 0 failed.**

**Comment findings.** The gate is clean. Worth recording that three *advisory*
comment defects were found by the code reviewer rather than the linter — all of
the "comment asserts a property the code does not have" class, which no linter
detects. See Code Review below; they are the reason several fixes in this diff
are comment changes.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

*Design review (before implementation):* `RR-B7U3I8` (critical, addressed) ·
`RR-VFI1W0` (significant, addressed) · `RR-R5VL59` (significant, addressed) ·
`RR-ZT9DXG` (significant, addressed) · `RR-TCZWUI` (minor, addressed)

*Code review (after implementation):* `RR-P0U6DI` (critical, addressed) ·
`RR-U186F1` (significant, addressed) · `RR-RQLNJ6` (significant, addressed) ·
`RR-34KJWO` (significant, addressed) · `RR-17HODW` (minor, addressed) ·
`RR-DDL5N1` (minor, addressed) · `RR-KF7419` (nit, addressed)

**All 12 addressed. No open responses at any severity.**

The two reviews each caught one genuine bug that the other phase could not have:

- **`RR-B7U3I8`** (design) — the planned `beforeEach`/`afterEach` counter
would have stranded the activity bar *permanently* on the cancelled navigations
`BUG-6C3V` documents as routine in Firefox. Confirmed by mutation test, not just
argument: reverting to the counter fails two tests.
- **`RR-P0U6DI`** (code) — the `minDuration` guarantee was not a guarantee.
A follow-up operation starting after a display period had elapsed inherited no
minimum and vanished on settle; worse, the intermediate settle hid the indicator
outright and the restart re-paid the full delay, producing a visible flicker on
a rapid double-save. Reproduced at production timings before fixing.

Three comments asserting untrue properties were corrected (`RR-U186F1`,
`RR-RQLNJ6`, and the `flush: 'sync'` justification). That was the reviewer's
closing point and it is the right one: a confident comment describing behaviour
the code does not have is how the next person inherits a bug they never thought
to look for.

**Unrelated changes:** none. The ten blank-line cleanups (`RR-KF7419`) are
tidy-ups of this diff's own deletions.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|---|---|---|
| AC1 sub-delay shows nothing | **PASS** | Unit (fake timers) + e2e: a local-server search and navigation never set `data-pending` and never make the bar visible. |
| AC2 delay + minimum | **PASS** | Clock walked: hidden at 499ms, shown at 500ms, held through 800ms, cleared at 900ms. Plus the two `RR-P0U6DI` regressions. |
| AC3 button width fixed | **PASS** | **Measured in Chromium**: resting width === in-flight width; pending label proven wider; hidden box non-zero. Mutation-verified. |
| AC4 paired equal width | **PASS** | **Measured**: Settings Save and Reset have identical widths. |
| AC5 no stranded bar | **PASS** | Real router driven through cancelled/aborted/duplicated/redirect/overlapping cases; four-route burst in-browser. Mutation-verified. |
| AC6 silent revalidation | **PASS** | Pagination tests assert no spinner while held rows stay on screen. |
| AC7 pagination holds rows | **PASS** | `EntityList.test.ts`, mutation-verified against `placeholderData` removal. |
| AC8 reduced motion | **PASS**, scope corrected | `.spinner` verified via `emulateMedia`. The other four are scoped and now carry their own rules — the original global claim was wrong (`RR-RQLNJ6`). |
| AC9 accessibility | **PASS** | `aria-disabled` present, native `disabled` absent; click, Enter and Space all suppressed, including in the pre-delay window. Keyboard test mutation-verified after being found tautological. |

**AC8 is the one whose *claim* changed** rather than its status: the acceptance
criterion is met, but only because suppression moved into each scoped component.
The original "one global rule covers everything" framing was not achievable
given Vue's scoped-CSS specificity.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** `DOCS-Q4MB31`

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Six commits, each independently revertable, each explaining the reasoning rather
than restating the diff. Probe files created during investigation
(`__probe.test.ts`, `__spinner-probe.spec.ts`, `__mintest.test.ts`,
`__flush.test.ts`) were all removed; working tree verified clean.

`frontend/CLAUDE.md` is what makes this usable by the next developer: the three
classes, which primitive per class, the governing rule, and both sanctioned
exceptions — without it, someone hand-rolls indicator #19 or "harmonises"
autosave's missing entry delay and makes it feel broken.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- not yet raised; branch tkt-tfsnby-loading-indicators is ready -->
