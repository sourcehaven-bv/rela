---
id: REV-MCDP
type: review-checklist
title: 'Review: Inline backtick-triggered entity-reference autocomplete in markdown editor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just test` — all Go packages green (no Go changes)
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — no boundary warnings
- [x] `just coverage-check` — 75.5% total, all package floors satisfied
- [x] Frontend `npm run test:run` — 738 tests / 41 files pass
- [x] Frontend `npm run typecheck` — clean
- [x] Frontend `npm run lint` — 75 pre-existing warnings, 0 errors
- [x] e2e `npm run typecheck && npm run lint` — both clean
- [x] e2e full suite — 197 passing, 1 skipped (unchanged baseline)
- [x] `just build` — all three binaries build cleanly

## Code Review

- [x] Ran the cranky-code-reviewer agent against the full diff
- [x] All findings filed as `review-response` entities and linked via
`has-review-response`

**Findings summary** (13 total from cranky review):

| Severity | Count | Addressed | Deferred |
|----------|-------|-----------|----------|
| critical | 1     | 1         | 0        |
| significant | 7 | 6         | 1 (RR-AKDU — DeepReadonly cast in popup, doc'd) |
| minor | 4      | 1         | 3 (RR-WUB2 fullscreen re-anchor, RR-Q2S8 scroll-tracking, RR-BJ5U phase-tearing) |
| nit   | 1      | 0         | 1 (RR-W5FU UTF-16 surrogate pair) |

**Critical (1) — addressed:**
- **RR-E25Z** — Cursor moves past the typed range no longer leave a zombie session.
Composable now tracks `expectedCursorCh`; cursorActivity closes the session on
jumps past it. Regression test added.

**Significant (6 addressed):**
- **RR-RH10** — Extracted `applyTypedToPhase` as the single source of truth
for phase transitions; the `as Phase` casts are gone.
- **RR-L56D** — `tryExactPrefixMatch` now disambiguates by longest-prefix
match instead of first-match alphabetical.
- **RR-HUNK** — Manual-type filter uses startsWith on label (not includes).
- **RR-UNAK** — Empty phase-2 query calls `listEntities` (typed listing
with default sort) instead of `searchEntities('*')`.
- **RR-OCSO** — `CodeMirrorLike` now uses typed overload signatures; all
five `cm.on/off` calls lost their `as never` casts.
- **RR-1629** — e2e exposes `useFastAutocompleteDelay()` helper backed by
a window flag the composable honors; all 6 e2e tests now use a 30 ms delay so
they don't race the production 600 ms grace period.

**Significant (1 deferred):**
- **RR-AKDU** — DeepReadonly cast in popup is a pragmatic workaround for
Vue's recursive readonly transform colliding with Entity's mutable nested types.
Documented inline; the popup is read-only. Refactor needs a project-wide
approach beyond this ticket's scope.

**Minor / nit:**
- **RR-BWXS** — Added doc comment to `CodeMirrorLike.getTokenAt`
explaining the `precise=true` requirement.
- The rest are deferred with documented reasons covering future polish
(scroll-tracking, fullscreen re-anchor, phase-tearing guards, UTF-16 surrogate
handling).

## Acceptance Verification

| AC | Status | Evidence |
|----|--------|----------|
| 1 (trigger in prose) | PASS | e2e: `typing a backtick in prose opens the popup`; unit: `opens in prose context after the open delay` |
| 2 (suppress in code contexts) | PASS | unit: 4 cases (fenced/url/closing-backtick/non-backtick); e2e: `typing a backtick inside a fenced code block does NOT open` |
| 3 (open delay) | PASS | unit: `cancels the open when a non-ID character is typed during the delay`, `cancels the open when Escape is pressed during the delay`; e2e: ``typing `flag-name` quickly does NOT show the popup`` |
| 4 (phase 1 — prefix list) | PASS | unit: `buildPrefixList` covers id_prefix, id_prefixes, id_type:manual; `filters prefix list by typed substring` |
| 5 (phase 2 — id list) | PASS | unit: `transitions to phase id when typed text equals a prefix`, `uses searchEntities once the partial id query has characters` |
| 6 (keyboard nav) | PASS | unit: `ArrowDown wraps the highlight`, `Escape dismisses`, `passes through non-navigation keys`; e2e: `Escape dismisses the popup` |
| 7 (non-focus-stealing) | PASS | popup's mousedown preventDefault keeps editor focus; verified in `passes through non-navigation keys` test |
| 8 (insert via insertEntityRef) | PASS | unit: `inserts the prefix and transitions to phase id`, `manual-id prefix jumps to phase 2 without inserting text`; e2e: round-trip test |
| 9 (auto-dismiss) | PASS | unit: `closes on space typed`, `closes when cursor moves off`, `closes on blur`, `closes when cursor jumps past the typed-after-trigger range` (RR-E25Z) |
| 10 (coexistence with toolbar) | PASS | TKT-I5NO suite still passes; e2e round-trip uses the same `insertEntityRef` helper |
| 11 (unit tests) | PASS | 24 Vitest cases in `useBacktickAutocomplete.test.ts` |
| 12 (e2e) | PASS | 6 e2e cases all green |

## Verification Evidence

- **Composable:** 24 Vitest cases covering trigger detection, open delay,
phase transitions, keyboard nav, auto-dismiss, cursor-jump detection,
setHighlight clamping, pick (prefix/manual), dispose, and the
searchEntities-vs-listEntities branching.
- **e2e:** 6 Playwright cases — all green, using the new
`useFastAutocompleteDelay` helper to avoid timing flakiness.
- **Real-browser validation** via Puppeteer confirmed the composable's
behavior end-to-end against production CodeMirror + EasyMDE.
- **All standard checks pass** (lint, typecheck, arch-lint, coverage,
build).
