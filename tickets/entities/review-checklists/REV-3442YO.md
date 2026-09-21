---
id: REV-3442YO
type: review-checklist
title: 'Review: Section sort: plus one declared order per enum, on every sort path'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `internal/filter`, `internal/dataentry`,
      `internal/dataentryconfig`, `internal/queryplan` and `internal/store/...`
      all green. The full `pgstore` conformance suite passes against a real
      PostgreSQL 18 (67s), so the postgres-gated tests actually ran rather than
      skipping.
- [x] Lint clean — `golangci-lint` 0 issues across every changed package.
      `arch-lint` clean, which matters here: `dataentry → filter` is a new
      package edge and the boundary check accepts it.
- [x] Comment lint gate clean — `just comment-lint`, no unresolvable doc links
      across 15,164 comments. Two were found and fixed during the work, both
      the same mistake: a `[Bracketed]` reference to something Go cannot link
      (a renamed symbol, then an unexported method).
- [x] Coverage maintained — `just coverage-check` PASS on both thresholds
      (package 50%, total 65%); total 79.9% (41807/52311). Changed packages:
      `dataentry` 83.0%, `dataentryconfig` 91.2%, `filter` 90.1%, `queryplan`
      88.4%, `store` 76.8%. Note the desktop cgo build failure that blocked
      this gate on TKT-M0WMEE did not recur.

**A coverage run failed and it was my fault, not the code's.** The first
attempt died on `parsing profile file: line "291.4 1 2" doesn't match expected
format`. That is a TRUNCATED line in `coverage.out`, not a threshold failure:
I had two `coverage-check` invocations running at once, both writing the same
profile. Diagnosed by finding the malformed line (42028, a fragment of a
`write_handler.go` block), confirming two writers, then stopping every test
process and re-running once. Worth recording because the error text points at
the profile parser and reads like a toolchain bug.

**Advisory comment findings: none introduced.** `just comment-report` reports a
`duplication` finding in `internal/store/graphquery.go` at :31/:49. That is
pre-existing — this change starts at :125. No suppressions were added anywhere
in the diff.

## Code Review

- [x] Run `/code-review` — cranky-code-reviewer, after a `/design-review` on the
      plan before implementation. Both found real defects that would have
      shipped.
- [x] All critical review-responses addressed — RR-6F2UF2, RR-C4QYTO, RR-TXFI2O,
      RR-Z7V8PI (design); RR-TC3ZLI, RR-S0H0I8 (code).
- [x] All significant review-responses addressed — RR-D1QOQ7, RR-PGEEZX,
      RR-QUQ0OE, RR-I3QG9P (design); RR-BEJAQM, RR-PD5JFT, RR-Y5H1SO (code).
- [x] Self-reviewed the diff for unrelated changes — 24 files against
      `origin/develop`, all TKT-9OFGH4. See the note on RR-B2AEVX below.

**Review Responses (15):**

| ID | Severity | Summary |
| --- | --- | --- |
| RR-6F2UF2 | critical | Descending path was an invalid comparator; reversed secondary keys |
| RR-C4QYTO | critical | Strings sorted natsort in Go vs byte-wise in SQL (99% of real titles differ) |
| RR-TXFI2O | critical | Bound-parameter `CASE` loses the index on a generic plan (4 → 1,915 buffers) |
| RR-Z7V8PI | critical | AC1/AC3/AC4 would have passed with the defect live |
| RR-TC3ZLI | critical | `sort:modified` silently returned id order |
| RR-S0H0I8 | critical | `ViewSection.Sort` validated and documented but never read |
| RR-D1QOQ7 | significant | Non-ISO dates and mixed date/datetime diverged across the boundary |
| RR-PGEEZX | significant | `sort=id` / `sort=modified` changed v1 API behaviour as side effects |
| RR-QUQ0OE | significant | List-valued and undeclared properties silently stopped sorting |
| RR-I3QG9P | significant | `ViewSection.Sort` level was ambiguous; nested cap is per-parent |
| RR-BEJAQM | significant | Nested-section docs contradicted themselves in the same file |
| RR-PD5JFT | significant | Section sort refused `modified` by accident, not by rule |
| RR-Y5H1SO | significant | SQL/index equivalence held by coincidence, not by test |
| RR-AGY8P7 | minor | Two comments pointed at the wrong mechanism |
| RR-B2AEVX | minor | **wont-fix** — reported unrelated churn was a stale local `develop` ref |

**The pattern both code-review criticals shared** is worth naming, because it
is the one to watch for next time: *documentation updated as though the code
worked.* `sort:modified` was documented, parsed, and parser-tested while the
sort silently returned id order; `ViewSection.Sort` was validated, documented
twice, and read by nothing. Both were invisible to every test in the diff,
because the tests that existed tested adjacent things — the parser rather than
the sort, the config rather than the render.

**One finding disputed.** RR-B2AEVX claimed TKT-Z4L0IU churn was riding along.
It is not: that commit is already merged upstream (PR #1615) and my *local*
`develop` ref lagged. `git diff origin/develop...HEAD` is 24 files, all this
ticket. Recorded rather than silently dismissed, because the reviewer's method
was right and anyone re-running the command on a stale checkout sees the same
phantom.

**Three defects I found in my own work**, listed because they are the ones no
reviewer flagged:

1. The property NAME had to be a literal alongside the values. Binding it alone
   was enough to lose the index under a generic plan.
2. The duplicate-enum-value case: `buildEnumIndex` let the last occurrence win,
   SQL's `CASE` takes the first. Reachable by a typo nothing rejects.
3. Grouping re-sorted each group by id, which would have discarded an author's
   `sort:` one group at a time — found while wiring RR-S0H0I8's fix.
4. `internal/store/graphquerynaive` had **no test file at all**, despite being
   the ordering used by sqlite, fsstore and memstore — three of the four
   backends. Its ranking was verified only indirectly, through the dataentry
   differential test. Now pinned directly (`order_test.go`), including the
   duplicate-value case, and mutation-tested. The package is deliberately excluded
   from the coverage floor (`.testcoverage.yml:107` — it is exercised through
   every backend's tests and cross-package coverage is not attributed without
   `-coverpkg`), so the gap was invisible to the gate by design; the exclusion
   is about attribution, not about the package not needing tests.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
| --- | --- | --- |
| 1 pushed == Go, page for page | PASS | Differential harness over 7 enum/id shapes × 2 page sizes × 3 pages; mutation (drop the rank from the pushed query) fails on page 1 |
| 2 declared order on both paths | PASS | Fixtures declare orders that reverse alphabetical, so the paths cannot agree by luck |
| 3 descending is a valid ordering | PASS | Original defect reproduced first (13 equal keys → reversed; secondary key inverted), then fixed |
| 4 schema edit rebuilds the index | PASS | `listIndexName` differs across no-values / declared / reordered / inserted |
| 5 two value orders don't collide | PASS | Declared values are in `StaticIndexSpecs`' dedup key |
| 6 index survives a generic plan | PASS | Real PostgreSQL 18, both directions, descending via backward scan |
| 7 section sort before the caps | PASS | Multi-parent fixture exhausts the shared budget; fails if it emits no parent with 3+ children |
| 8 edge semantics match SQL | PASS | Nulls both directions, undeclared properties, list values, non-ISO dates |

**Every criterion is mutation-tested.** The table in IMPL-Z23EHR names the
mutation for each and what it breaks. One mutation was found to be **inert** —
inverting a non-equal comparison is still correct under a single-pass
comparator — and was replaced with one that reproduces the original multi-pass
architecture. That is the check worth repeating: confirm the mutation compiled
and actually applied before believing a pass.

**The SQL/index equivalence is now fuzzed**, not just tabled:
`FuzzOrderSQLRankMatchesIndex` ran 450,000 executions with no divergence. It
converts "the two generators currently agree" into a property, which is what
the reviewer actually asked for.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` — DOCS-JATRMW
- [x] User-facing documentation updated — `docs/data-entry.md`,
      `docs/metamodel.md`, `docs/postgres-backend.md`, all edited at their
      `docs-project/` source and regenerated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-JATRMW

## Final Checks

- [x] Commit message explains the why, not just what — each commit states the
      decision or the measurement behind the change, not the diff.
- [x] No TODOs or FIXMEs left unaddressed — none introduced. (A grep hits
      `VTODO` in `config.go`; that is iCalendar, pre-existing.)
- [x] Ready for another developer to use — the ordering rule is documented once
      on `filter.QuerySort` and cited from the places that implement it, so the
      next person changing a sorter finds the constraint before the bug. The
      two SQL generators carry explicit warnings against the "safer-looking"
      change that would silently cost the index.

**Three user-visible behaviour changes ship with this**, all in DOCS-JATRMW's
release-note item: enum-sorted lists move to declared order; string sorts
become byte order (99% of this repo's own titles move); `sort=id` becomes byte
order on the API while the CLI keeps natural order.

## Post-PR: rebase onto a moved develop

The PR opened with `mergeable=CONFLICTING` — 24 commits landed on develop while
this work was in review, several touching the same files. **GitHub does not run
`pull_request` workflows on a conflicting PR**, so only CodeQL reported and the
main CI slate never appeared. That is worth knowing because the PR page showed
green CodeQL checks and no failures, which reads identically to "CI passed"; an
empty commit to re-trigger did nothing, because the cause was the conflict, not
a missed event.

Resolved by rebasing onto `origin/develop`. One conflict, in
`dataentryconfig/config.go`: develop added `ViewSection.Create` (TKT-R4BMJM's
opt-in create affordance) where this branch added the three sort keys. Both are
purely additive to the same struct, so both were kept.

Re-verified on the new base: all packages green, docs regenerate identically,
and the three new doc sections survived.

**One test failure during that check was a stale local build artifact, not a
regression.** `TestAppEditorBundleEmbedded` failed on
`editor stylesheet must not declare an @font-face`. `internal/dataentry/app_editor_dist/`
is gitignored and held a Sept 13 EasyMDE build, from before the Milkdown swap
(TKT-D2JML7) removed the Font Awesome webfont. Confirmed by running the same
test in a clean worktree of `origin/develop`, where it passes by skipping — the
artifact is absent there. Deleting the stale files fixed it; CI builds the
bundle fresh and was never affected.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` gates
      on the ticket already being `done`, so the PR post-dates this checklist —
      see TKT-UFV01M)

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
