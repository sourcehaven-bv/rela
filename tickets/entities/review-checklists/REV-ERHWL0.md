<!-- @managed: claude-workflow v1 -->
---
id: REV-ERHWL0
type: review-checklist
title: 'Review: Memoize dashboard breakdown and table-row derivation'
status: pending
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

Go: `just test` no failures; `just lint` 0 issues; `just coverage-check` package
floor (50%) and total (65%) both PASS at 77.6%. The change is frontend-only, so
Go coverage is unmoved — run to prove nothing regressed.

Frontend: 1667 tests / 105 files pass, `vue-tsc --noEmit` clean, `npm run lint`
0 errors with no warnings in the changed files, `npm run build` clean.

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

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** Not yet opened. `rela validate` is blocked repo-wide by BUG-E9DYW5
(`status: done`, no `has-review`), introduced by #1314 and unrelated to this
ticket. Three open PRs fix it — #1328, #1330, #1335 — each also repairing the
malformed `AM-feed-field-redaction.md` frontmatter that was masking the
violation. This PR opens once any of them merges.
