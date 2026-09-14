---
id: REV-CTO2PT
type: review-checklist
title: 'Review: store: paged variant of GraphQueryer (GraphQueryPage) — pgstore SQL pushdown + naive early-stop + conformance'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just test` — full suite clean; `-race` clean on the changed packages
- [x] `just lint` — `golangci-lint run ./internal/store/... ./internal/acl/...` → **0 issues**
- [x] `just coverage-check` — PASS (package floor + total; total 76.6%)
- [x] `just arch-lint` — OK (after allowing `graphquerynaive` → `storeutil`, the shared cursor codec)
- [x] `just plimsoll` — OK (+1 method per store, documented as interface-tracking)
- [x] `go build` clean under all three backend tags (default / `postgres` / `memorybackend`)
- [x] **pgstore against live PostgreSQL 15.17** — `RELA_TEST_DATABASE_URL` was unset locally, so
the pgstore suite would have silently skipped. Stood up a scratch DB and ran it
for real: `ok ... 27.208s` under `-race`, with **15 paged subtests confirmed
PASS (not SKIP)**. DB dropped after.

## Code Review

- [x] `cranky-code-reviewer` invoked on the full diff
- [x] review-response entity created per finding
- [x] all critical/significant findings addressed

The reviewer explicitly confirmed the keyset walk itself is correct — it ran an
exhaustive 32-combination placeholder/arg consistency check across every
predicate permutation (no desync), probed odd id orderings against a real
memstore, and verified cursor identity uses the last *emitted* row rather than
the lookahead. Findings were about **undefended** correctness, not broken
correctness.

| ID | Severity | Finding | Status |
|----|----------|---------|--------|
| RR-L2AC9E | critical | `RunPage`'s `matches()` error abort was untested (mutant survived the whole suite) and diverges from `Run`'s continue-on-error | addressed |
| RR-245QB2 | significant | `Cursor`/`Limit` on shared `GraphQuery` silently ignored by 3 of 4 methods | addressed |
| RR-R713PD | significant | `graphquerynaive.Reader` didn't state the ascending-id precondition `RunPage` now requires | addressed |
| RR-DUBJ5B | significant | Conformance seeds too tame to catch cross-backend ordering divergence | addressed |
| RR-M6JPEW | minor | `walkGraphQueryPages` guarded page-count but not cursor advance or duplicates | addressed |

Nits 5–7 from the review were also fixed rather than deferred: extracted
`storeutil.FinishPage` (collapsing three copies of the limit+1/truncate/encode
dance, including the pre-existing one in `pgstore.ListEntitiesPage`), dropped a
near-vacuous `NotContains` assertion, and pre-sized the item slices.

**Every fix was mutation-verified** — re-breaking the implementation and
confirming a test now fails:

- swallow `matches()` error → `TestRunPagePropagatesMatchError` FAILS (previously survived)
- case-insensitive keyset compare → `Page_hostile_ids_match_unpaged_order` FAILS on **both** fsstore and memstore (previously undetectable)
- un-normalize the negative limit → `Page_negative_limit_treated_as_unbounded` FAILS

### Bug found and fixed during review follow-up

Extracting `FinishPage` and adding the reviewer's pre-sizing nit introduced a
**real panic**: `make([]T, 0, want)` with `Limit: -1` gives a negative capacity
— `makeslice: cap out of range`. The existing
`Page_negative_limit_treated_as_unbounded` conformance subtest caught it
immediately, which is precisely the edge case planning called for. Fixed by
normalizing `limit := max(q.Limit, 0)` once at the top of **both** `RunPage` and
pgstore's `GraphQueryPage` (the pgstore copy had the same latent exposure), so a
negative limit can never reach either the capacity hint or the page logic.

This is the clearest evidence the suite is doing its job: a nit-driven
micro-optimization introduced a crash, and the conformance test failed within
seconds.

## Acceptance Verification

Criteria from PLAN-ZYYWJ7, each mapped to a named conformance subtest run on all
three backends:

| # | Criterion | Evidence | Result |
|---|-----------|----------|--------|
| 1 | `Limit == 0` → full set, empty cursor | `Page_full_when_limit_zero` | PASS |
| 2 | `Limit > 0` → ≤ Limit items, cursor iff more | `Page_respects_limit_and_sets_cursor` | PASS |
| 3 | Exact fit → no cursor | `Page_no_cursor_when_limit_exactly_exhausts` | PASS |
| 4 | Walk yields each match once, stable order | `Page_walk_yields_every_match_once`, `Page_stable_order_across_calls` | PASS |
| 5 | Opaque cursor; malformed → error not restart | `Page_invalid_cursor_returns_error` | PASS |
| 6 | Cursor past end → empty page | `Page_cursor_past_end_returns_empty` | PASS |
| 7 | Composes with every predicate shape | `Page_composes_with_InheritThrough`, `Page_hostile_ids_match_unpaged_order` | PASS |
| 8 | `total` still reachable | `GraphCount_matched_and_total` (unchanged); scope-changed — `Total` deliberately not on the page | PASS |
| 9 | All three backends satisfy 1–8 | `RunAll` → fsstore + memstore + **pgstore on live DB** | PASS |

Criterion 8 changed by user decision during implementation: `Total` was dropped
from the page because a sometimes-meaningful total is worse than none, and
`GraphCount` already returns `(matched, total)` — strictly more than a page
could report. Ticket note 2 is still satisfied (`total` remains available); it
just doesn't live on the page. The upside is that `RunPage` can now genuinely
stop early, since it no longer needs to visit every entity to count.

Reporter note 1 (stale `graphquery.go:17` comment claiming pgstore SQL-pushdown
was a pending follow-up) fixed.

## Review Summary

Ready to merge. The design is stronger than what went into review: the
`GraphPageQuery` split means the compiler now enforces that paging fields exist
only where they're honored — setting a limit and calling `GraphCount` or
`MatchingIDs` is no longer expressible. That was free to do only because no
production consumer of `GraphQueryPage` exists yet (verified by grep); after
TKT-YWDGZD lands it would have been a breaking change.

No open critical or significant review responses.
