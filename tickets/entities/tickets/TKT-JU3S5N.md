---
id: TKT-JU3S5N
type: ticket
title: 'store: paged variant of GraphQueryer (GraphQueryPage) — pgstore SQL pushdown + naive early-stop + conformance'
kind: enhancement
priority: medium
effort: m
status: wont-fix
---

## Superseded

**Closed `wont-fix` on 2026-09-12 — superseded by TKT-1U8XYN (#1526), which
shipped paging for `GraphQueryer` while this branch was unmerged.**

The capability this ticket asked for exists on `develop`. The symbols differ, so
a name search suggests otherwise: there is no `GraphQueryPage`, no
`GraphPageQuery` and no `graphquerynaive.RunPage`. What landed instead is

- `OrderBy []OrderSpec`, `Limit int` and `Offset int` directly on
`store.GraphQuery`,
- `graphquerynaive.Order` / `graphquerynaive.Page` for the delegating backends,
- `ORDER BY` / `LIMIT` / `OFFSET` pushed into the pgstore CTE builder,
- paging conformance in `storetest/graphquery.go` (`WindowEqualsSlice`,
`CountIgnoresPaging`, `CountMatchedEqualsGraphCountMatched`,
`HeadersPageTheSame`), so every backend is held to it, and
- a live consumer in `internal/dataentry/listpushdown.go`.

Both designs bound the same surface, so landing this one would have left two
paging mechanisms on `GraphQueryer` at once. They also disagree about where the
paging fields belong, and that disagreement is not resolvable by merging: this
ticket's premise (recorded in RR-245QB2) is that `Limit` must NOT sit on
`GraphQuery`, because `GraphCount` and `MatchingIDs` cannot honour it and
silently ignoring it is an ACL footgun. TKT-1U8XYN put the fields there anyway
and documented the ignoring as intended — "GraphCount and MatchingIDs IGNORE all
three" — pinned by its `CountIgnoresPaging` conformance test. `GraphQuery`
cannot document `Limit` as both obeyed-and-ignored and forbidden-here.

The merged design is therefore the one that stands. The keyset argument is not
wrong, but it is an argument about a different operating point; it has been
carried over to TKT-8AUD1U for whoever bounds the dashboard cards.

## What survives

- **The keyset rationale**, moved to TKT-8AUD1U: keyset is stable under
concurrent writes and cheaper at depth; offset supports arbitrary `OrderBy` and
random access, which the list UI needs.
- **The hostile-id conformance technique** (RR-DUBJ5B): seeding ids where
insertion, numeric and byte order all disagree (`tkt-3`, `TKT.9`, `TKT-10`,
`TKT-2`, `TKT`, `TKT-Z`, `TKT-a`) and comparing a paged walk against the RAW
iterator rather than a sorting helper. Worth reusing if a cursor is added later,
since ordering divergence is invisible to well-behaved seeds.
- **The ordering-precondition finding** (RR-R713PD): any keyset walk over
`graphquerynaive.Reader` depends on ascending-id `ListEntities`, which the
consumer-side interface cannot enforce at compile time.
- **The error-handling argument** (RR-L2AC9E): once results are bounded, a short
page with an empty `NextCursor` is indistinguishable from end-of-results, so
skipping a row on the ACL read path silently reads as "these entities do not
exist". Applies to any future paged read, not just this one.

## Process note

TKT-8AUD1U carried a sequencing note naming this branch and asking implementers
to "build on that rather than inventing a second bounding mechanism. Re-check
whether it has merged before starting." That check did not happen before
TKT-1U8XYN, which is why two designs for one capability existed. Recorded so the
next person understands the history, not to reopen the decision — TKT-1U8XYN is
merged, consumed and measured.

## Original content

The original ticket body, the full design rationale and the five review
responses live in `git log` on branch `tkt-ju3s5n-graphquery-paging` (commit
`f83217ff`), which was left unpushed. PLAN-ZYYWJ7 and REV-CTO2PT retain the
planning and review record.
