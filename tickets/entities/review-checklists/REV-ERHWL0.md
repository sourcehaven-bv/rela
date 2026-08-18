<!-- @managed: claude-workflow v1 -->
---
id: REV-ERHWL0
type: review-checklist
title: 'Review: Memoize dashboard breakdown and table-row derivation'
status: done
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

Go: `just test` no failures; `just lint` 0 issues; `just coverage-check` package
floor (50%) and total (65%) both PASS at 77.7%. The change is frontend-only, so
Go coverage is unmoved — run to prove nothing regressed. `just arch-lint` and
`just plimsoll` clean.

Frontend: 1674 tests / 106 files pass, `vue-tsc --noEmit` clean, `npm run lint`
0 errors with no warnings in the changed files, `npm run build` clean.

Re-verified after rebasing onto develop across the Pinia 3→4 major bump,
including re-running the mutation tests.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-DBKEY1 (critical), RR-DBTEST2 (significant),
RR-DBEAGER3 (significant) — all `addressed`.

The critical finding was real and I reproduced it independently before acting on
it: keying the derived data by `cardKey` rendered one card's breakdown on
another card's tile. The fix restructures to a per-card view model rather than
widening the key, which removes the collision class instead of avoiding one
instance of it.

Three reviewer suggestions were NOT taken, deliberately:

- *Hoist a shared frozen empty array for the `|| []` fallbacks* (nit) — the
  fallbacks no longer exist; the view model always supplies a real array.
- *Build the map once in `loadData` after `Promise.all` settles* — that couples
  derivation to fetching, so a config-only change (e.g. a different `group_by`)
  would not re-derive. The reactive form is correct; the O(cards²) concern is
  bounded by cards-per-dashboard, which is single digits.
- *`type Breakdown` sits under the doc comment* (nit) — resolved incidentally:
  the comment now sits on `cardViews`, and `Breakdown` is declared above it.

Diff self-review: two files, both in scope. No unrelated changes. `getCardCount`
was deleted because the view model supplies the count — verified it has no other
caller.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 — breakdown derived once per card per render: PASS.** Counting getter
  yields 3 reads for 3 entities across two template call sites. Mutation-tested:
  the pre-change form gives 9.
- **AC2 — table rows derived once per card per render: PASS.** Sorted card
  renders `['a','b','c']` and stays under the 15-read pre-change bound.
  Mutation-tested: the pre-change form gives 21. (This criterion was initially
  signed off on a test that could not fail — see RR-DBTEST2.)
- **AC3 — no change to rendered output: PASS.** The 7 pre-existing #1316 cases
  pass unmodified. Note AC3 as originally written was too weak: it would have
  been satisfied by the RR-DBKEY1 collision, since no existing test covered two
  cards sharing a key. The added regression test closes that gap.

Every load-bearing guard was mutation-tested — each was broken individually and
confirmed to fail: collision keying, display guards, and both double-derivation
counters.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: `kind: refactor`,
      no user-facing surface changes — the `done-enhancement-needs-docs-done`
      gate does not apply)
- [x] ~~User-facing documentation updated~~ (N/A: internal refactor)
- [x] ~~Docs-checklist marked as done~~ (N/A: internal refactor)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

The godoc on `cardViews` records why derived data must not be keyed by
`cardKey`, so the collision is not reintroduced by someone consolidating the two
keying schemes.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] ~~All CI checks pass~~ (N/A: every job this branch can affect passes;
      "Rela Tickets" is red on develop for unrelated tickets — see below)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1361

All checks pass except **Rela Tickets**, which runs
`rela validate --project tickets`. That job is red on `develop` itself: a clean
`develop` worktree reports the same 10 errors, none from this branch —
TKT-UIR41P missing `has-docs`/`has-review`, three open `RR-*` blocking merge,
and three bugs missing `has-review` (one also missing a description).

The 11th error was this ticket's own `has-review` gate, which fired because this
checklist stayed `pending` until the PR existed — the PR boxes could not be
honestly checked before then. Resolved by completing it now that #1361 is open.

The earlier blocker (BUG-E9DYW5 + the malformed `AM-feed-field-redaction.md`
frontmatter masking it) was fixed on develop by #1337, not by any of the three
dedicated PRs — #1335 was closed, #1328 and #1330 are now redundant.
