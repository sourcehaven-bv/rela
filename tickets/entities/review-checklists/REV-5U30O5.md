---
id: REV-5U30O5
type: review-checklist
title: 'Review: Query-budget test for the nested view section'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `internal/dataentry` and `internal/store/storetest` green,
      and both green again under `-race -shuffle=on` (457s / 2.4s), which is the
      combination that would expose the `findViewByEntityType` nondeterminism if
      the new view had shared an entry type.
- [x] Lint clean — `golangci-lint` 0 issues on both changed packages. Two
      rounds of `misspell` fixes (British spellings: `programme`, `favour`,
      `neighbours`).
- [x] Comment lint gate clean — `just comment-lint`, no unresolvable doc links
      across 15,015 comments. `just comment-report` shows no advisory findings in
      either changed file (266 exist repo-wide, none introduced here).
- [x] Coverage maintained — tests only add coverage, and `.testcoverage.yml`
      enforces floors without a ratchet. See the note below on the full-suite run.

**Note on the coverage run.** `just coverage-check` reported a `FAIL`, which
turned out to be `cmd/rela-desktop [build failed]` — pre-existing and local, not
from this change. Diagnosed rather than assumed: the same run with both changed
files stashed out fails identically, and the cause is in the log —

```
# runtime/cgo
You have not agreed to the Xcode license agreements.
```

The Wails desktop package needs cgo, which this machine cannot compile until
`sudo xcodebuild -license` is accepted. The same cause makes `just plimsoll`
skip locally. CI builds on Linux and is unaffected. Every other package in the
run passed, and `plimsoll` on the two changed packages is clean when invoked
directly.

## Code Review

- [x] Run `/code-review` — cranky-code-reviewer, after a `/design-review` on the
      plan before implementation. Both found real defects.
- [x] All critical review-responses addressed — RR-3RRVYB, RR-6D2GN6.
- [x] All significant review-responses addressed — RR-17V5JE, RR-3EK9TV,
      RR-FGR60O.
- [x] Self-reviewed the diff for unrelated changes — two files, both test-only.
      `git diff` on `internal/dataentry/sections_nested.go` confirmed empty after
      every mutation experiment.

**Review Responses:**

| ID | Severity | Status | Summary |
| --- | --- | --- | --- |
| RR-3RRVYB | critical | addressed | Flat limit left 84% slack; a 1,200-row over-fetch passed. Bound is now a function of the emitted tree. |
| RR-6D2GN6 | critical | addressed | `Breadth.Tx` shared maps across separate mutexes — a real data race (8 reports under `-race`). |
| RR-17V5JE | significant | addressed | Comment claimed `ListEntityHeaders` carries 280k ids; it carries one (target ids are deduped). |
| RR-3EK9TV | significant | addressed | `Breadth` ignored `MatchingIDs`, the ACL membership-walk batch. |
| RR-FGR60O | significant | addressed | `Calls`/`Totals` were unused API; `String()` also read the maps under two separate lock acquisitions. |
| RR-ENUHES | minor | addressed | Fixture asserted only the upper half of the straddle. |
| RR-Y8PRYR | minor | addressed | Test-3's comment claimed four validation rules it cannot verify. |
| RR-P72CH2 | minor | addressed | `nestedSectionBudget` comment stated a counterfactual no committed test pins. |
| RR-RTNTSR | minor | addressed | `budgetEpics` collapses the parent level below n=5. |
| RR-CNQJVM | minor | addressed | Fixture was larger and slower than the straddle required. |

**Two design-review findings deserve naming**, because both were cases where a
test would have shipped that passes regardless of the defect:

1. The planned AC3 test asserted on `planNestedRows`' output. It is a pure
   function and cannot observe what its caller does with the result, so it
   passed with the defect reintroduced — and duplicated an existing test.
2. The fixture held the parent level at one row, so a per-parent N+1 (the
   regression this ticket was written about) would not have moved the count.

**Follow-up filed:** TKT-DAD248 — merge `Counting` and `Breadth` into one
recorder, and fix `Counting.Tx`'s equivalent latent race. Out of scope here:
`Counting` has many call sites and this is a test-only ticket.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
| --- | --- | --- |
| 1 size independence | PASS | `TestQueryBudget_NestedSectionIsSizeIndependent` — 15 reads at n=10 and n=50, while parents scale 2→10 and children 10→50 |
| 2 pinned constant | PASS | `nestedSectionBudget = 15`, documented per-leg beside the four existing pins |
| 3 RR-HKHPYG guard | PASS | `TestQueryBudget_NestedRelationColumnsResolveOverEmittedRowsOnly` — mutation-verified: 6,624 ids against a 2,500 allowance with the defect, ~2,170 without |

**Mutation-tested sensitivity of AC3** (over-fetch injected into
`buildNestedTree`, production code restored after each):

| Injected over-fetch | Result |
| --- | --- |
| +300 rows | passes (inside the ~13% margin) |
| +600 rows | **caught** |
| +1,200 rows | **caught** (the flat constant missed this) |
| full defect (all visible children) | **caught** |

The residual margin is stated in the constant's doc comment rather than implied
away. It is the slack that keeps the test from failing on incidental fixture
drift; closing it entirely would mean pinning a measured total, which then moves
whenever the fixture does.

**Not an AC, added from review:** `TestQueryBudget_FixtureConfigIsValid` —
`newAppFromParts` publishes the schema directly and never calls
`ValidateConfig`, so all five budget fixtures could have pinned numbers for
configs no operator could load. Mutation-verified.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: `kind: test`)
- [x] ~~User-facing documentation updated~~ (N/A: no user-visible surface
      changes. The convention this satisfies is stated in root `CLAUDE.md`,
      which already requires a budget test for every new read path and needs no
      amendment.)
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit message explains the why, not just what — to be written at commit.
- [x] No TODOs or FIXMEs left unaddressed — `grep` over both changed files
      returns nothing.
- [x] Ready for another developer to use — `storetest.Breadth` is documented
      with its reason for existing beside `Counting`, what it does and does not
      measure, and why its `Tx` shape differs. Both temporary probe files and the
      `budgetConfigHook` seam used during measurement were removed.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` gates on the ticket already being `done`, so the PR post-dates this checklist — see TKT-UFV01M)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
