---
id: REV-FTHW8Q
type: review-checklist
title: 'Review: History/diff views: put selected versions in the URL so a diff is shareable'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] Frontend unit tests pass — `1429 passed (87 files)`
- [x] E2E tests pass — `239 passed` against real PostgreSQL
- [x] Typecheck clean — `vue-tsc` (frontend) and `tsc` (e2e)
- [x] Lint clean — 0 errors in both projects
- [x] Markdown lint clean on the changed doc
- [x] Prettier clean on all changed files
- [x] ~~`just test` / `just coverage-check` (Go)~~ (N/A: no Go source changed —
the only Go-adjacent edit is the e2e TypeScript fixture)

Verified against a `rela-server-postgres` binary built from this branch, with
the CI sweep intervals — not mocks. The e2e run includes the whole wizard suite
(`?step=N`, the closest sibling of this URL-sync pattern) and the two
pre-existing relation-history tests, so regression coverage is real.

## Code Review

- [x] `cranky-code-reviewer` invoked
- [x] Findings recorded as `review-response` entities
- [x] All critical/significant findings addressed

The reviewer independently re-implemented both views' control flow and attacked
the interleavings the plan flagged as risky — external nav between
`seedFromUrl()` and `publish()`, a slow load's publish racing a newer nav, rapid
selects, back-button restores. It found **no critical issues** and confirmed the
echo guard, the `recomputeSeq` interaction, and the array-getter fix are sound.

| ID | Severity | Finding | Status |
|---|---|---|---|
| RR-XM367L | significant | Relation bare URL published two frozen ordinals while the entity view published a live-relative link | addressed |
| RR-L4M1WK | significant | Empty version history left bogus params in the URL unvalidated | addressed |
| RR-1QWV9S | significant | `select()` did no type coercion — the `Side` invariant rested on a template binding | addressed |
| RR-W7L33A | minor | `publish()` passthrough; duplicated defaults in `HistoryView`; colliding e2e waiter names | addressed |

**Review summary.** RR-XM367L was the one users would actually have hit: the
headline feature is "share this diff", and it silently meant two different
things depending on which history you were looking at. The relation view's
default published `?base=2&target=3` (frozen) where the entity view published
`?base=3&target=current` (live) — so an otherwise-identical link would stop
meaning "the most recent edit" as soon as a new version landed, for relations
only. Fixed by returning the sentinel from `defaultSelection()`; the plumbing
(`sideState` resolving `current` to the newest version) already existed.

RR-1QWV9S is the kind of finding worth taking seriously even though nothing was
broken: the mutation test I ran proved the *current* wiring correct, not the
*next* caller. `coerceSide` now enforces the invariant on every write into the
refs, so the guarantee lives with the type rather than in a Vue template two
files away.

Fixes added 6 unit tests (39 → 45) and 1 e2e test (5 → 6 in this spec), each
pinning a specific fixed behaviour rather than restating the change.

## Acceptance Verification

Each criterion from PLAN-MWQHUZ, verified against a real postgres backend:

| AC | Verdict | Evidence |
|---|---|---|
| 1 Deep link selects the pair | PASS | `?base=1&target=2` → select values `1`/`2`, caption `v1 → v2`, timeline row 1 highlighted, no interaction |
| 2 Selection writes the URL | PASS | Asserted after dropdown change, timeline click, and swap; unit tests assert the `replace` payload |
| 3 Reload is stable | PASS | Non-default pair `1`/`3` survives `page.reload()` |
| 4 `current` round-trips | PASS | Parses to the sentinel, serializes unchanged, never coerced to a number |
| 5 No params = today's behaviour | PASS | Bare URL → `base=3&target=current`, then published explicitly |
| 6 Bad params degrade silently | PASS | e2e `?base=999&target=nonsense` → defaults, no error state; 14 unit cases incl. traversal and script-tag payloads |
| 7 No history spam | PASS | `replace` called, `push` never |
| 8 Restore resets | PASS | `load(false)` → `resetToDefaults()`; pre-existing restore e2e still green |

Two criteria emerged during the work and are also covered: selection writes
**merge** into the query (a `?return_to=` survives), and a stale ordinal is
**corrected** in the address bar rather than left to rot.

## Documentation

- [x] `docs/postgres-backend.md` — shareable-diff URLs documented for both
entity and relation history, including the live-relative meaning of `current`
and the fact that a shared link is not a capability (the recipient still needs
their own read permission)
- [x] Doc claim corrected during review — the fallback sentence now states that
the address bar is rewritten, which is what RR-L4M1WK made true

Placed in `postgres-backend.md` rather than `data-entry.md` because that is
where the history feature is actually documented; the new text sits directly
under the paragraph describing the history UI.
