---
id: IMPL-C36FHB
type: implementation-checklist
title: 'Implementation: Framework-level loading/pending indicator system for the data-entry SPA'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**What shipped** (5 commits on `tkt-tfsnby-loading-indicators`):

| Commit | Contents |
|---|---|
| `cb3e0359` | Planning: pagination anti-flash contract pinned + stale comment corrected |
| `a823a6d9` | Primitives: `useDelayedPending`, `useNavigationPending`, `ActivityBar`, `PendingButton`, `pendingTimings` |
| `52fd1e7e` | Migration: 6 action buttons → `PendingButton`; Settings action-pair equal width |
| `98ce76a8` | Ten duplicated `@keyframes spin` → one, plus one global reduced-motion rule |
| `24e6fe3e` | `frontend/CLAUDE.md` conventions + `docs/data-entry.md` user note |

**Edge cases from planning, all handled and tested:**

- Flapping source inside one delay window — no stacked timers (`if (delayTimer !== null) return`).
- Re-entry while visible — does not restart the minimum (would extend it indefinitely under rapid saves).
- Unmount mid-`DELAY`/`DISPLAY`/`EXPIRE` — `onScopeDispose` clears both timers; asserted via `vi.getTimerCount() === 0` (the `RR-YWWAL` leak class).
- Cancelled / aborted / duplicated navigation, redirect chains, overlapping navigations — all covered in `useNavigationPending.test.ts`.
- `delay: 0` / `minDuration: 0` — degrade to instantaneous with no timer armed.
- Non-finite / negative options — coerced to defaults rather than arming a broken timer.

**One design change during implementation.** The `watch` in `useDelayedPending`
needed `flush: 'sync'`. With Vue's default post-flush timing, a source that
flips true and settles within the same frame could have its cancellation land
*after* the timer was scheduled — producing a flash in exactly the case the gate
exists to suppress. Found because five tests failed in a way that looked like a
test-harness problem; it was a real ordering hazard.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Helpers rather than repetition: `gate()` (scoped composable construction),
`tick()` (advance + flush), `mountButton()` (typed prop defaults),
`makeRouter()` (real router with fixture routes). E2E selectors live in
`PendingPage` / `SettingsPage` per the enforced page-object rule — no raw
`locator()` in specs.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Automated: **1862 unit tests / 117 files pass**; **263 e2e pass** (8 skipped);
`vue-tsc` clean; eslint **0 errors**.

Verified in a real Chromium browser (`e2e/tests/pending-indicators.spec.ts`, 7
tests) because the load-bearing guarantees are geometric and jsdom has no layout
engine:

| AC | Evidence |
|---|---|
| AC1 (sub-delay shows nothing) | Local-server search + navigation assert `data-pending` never set and the bar never gains `--visible`. |
| AC2 (delay + minimum) | Unit tests walk the clock: hidden at 499ms, shown at 500ms, held through 800ms, cleared at 900ms. |
| AC3 (button width fixed) | **Measured**: resting width === width during flight; pending label proven *wider* than resting, and its hidden box measured non-zero. |
| AC4 (paired equal width) | **Measured**: Settings Save and Reset have identical `boundingBox().width`. |
| AC5 (no stranded bar) | Real router driven through cancelled/aborted/duplicated/redirect/overlapping navigations; plus a rapid four-route burst in-browser. |
| AC6 (silent revalidation) | Pagination tests assert no spinner while previous rows are held. |
| AC7 (pagination holds rows) | Pinned in `EntityList.test.ts`, mutation-verified. |
| AC8 (reduced motion) | **Measured** via `emulateMedia({reducedMotion:'reduce'})`: `animationName` is `spin` normally, `none` under the preference. |
| AC9 (a11y) | `aria-disabled` present / native `disabled` absent; click, Enter and Space all suppressed — including during the pre-delay window, where nothing is on screen and a user is most likely to click again. |

**Three mutation tests** — the tests were verified to fail against the wrong
implementation, so they pin behaviour rather than passing vacuously:

1. Remove `placeholderData` → pagination test fails (spinner returns).
2. Replace identity-tracking with the originally-planned counter → **the
`RR-B7U3I8` leak reproduces**: guard-cancelled navigation and redirect chains
strand the bar. The critical design-review finding was correct.
3. `visibility: hidden` → `display: none` → both e2e width tests fail.

Mutation 3 initially exposed a **weak test**: the first width assertion passed
under the mutation because "Search" happened to be wide enough already. The test
was strengthened to assert the pending label is genuinely wider and that its
hidden box is non-zero, then re-run against the mutation to confirm it now
fails.

**E2E flake, investigated not assumed.** Full-suite runs showed one or two
failures in *different* tests each run, all passing in isolation. Ran the full
suite on the pre-existing base commit `8acabe71`: **it fails 2 tests too**, in
yet another pair. Pre-existing parallel-load flake, unrelated to this work —
this branch is no worse. (One earlier failure *was* mine: a stale e2e binary
left over from a mutation test. Rebuilt and it passed.)

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

- Generalizes from `AutoSaveIndicator` (fixed box, opacity fade, sr-only live
region, empty-at-idle announcement) rather than rebuilding it; `useAutoSave` is
untouched.
- DRY: the ten `@keyframes spin` copies collapse to one — the single largest
duplication removed here. Timings are single-sourced in `pendingTimings.ts`.
`useDelayedPending` is deliberately *not* retrofitted into `useAutoSave`: that
has extra states (`saved`, `error`) and equivalent minimum semantics already, so
the merge would be churn against working, tested code.
- Security: presentation-only, no new I/O. The one real concern — `aria-disabled`
not preventing activation — is handled and tested on both input paths.
- Probe/debug files created during investigation were removed
(`__probe.test.ts`, `__spinner-probe.spec.ts`); working tree verified clean.
