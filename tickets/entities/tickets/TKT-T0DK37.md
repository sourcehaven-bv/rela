---
id: TKT-T0DK37
type: ticket
title: Bound the structured search path the way free-text already is
kind: enhancement
priority: medium
effort: s
status: backlog
---

## What

`executeQuery` (`internal/dataentry/helpers.go:399`) caps the free-text branch
at `maxFreeTextSearchResults = 1000` (`helpers.go:637`), but the structured
branch — `visibleListByTypes`, taken for `type:` / `prop:` queries — has **no
bound at all**. It appends every entity of every requested type into one slice.

Dashboard cards are all structured queries, so the one existing safety limit
does not cover the surface that actually loads the most.

## Why separately from the aggregation ticket

This is the cheap backstop, independent of any wire-format change: even after
count/breakdown cards stop fetching rows (TKT-AIEGHU), `/_search` remains
reachable directly with an unbounded `type:` query, and list views use the same
path. Worth having regardless of how the card work lands.

## Sequencing — check before starting

`rela-next-action` commit `c5960f9c` ("perf(dataentry): push equality property
filters into the store") **rewrites this exact function**, changing
`visibleListByTypes`' signature to take `[]store.PropPredicate` and splitting
out `visibleEntitiesOfType`. That materially reduces how much this path loads
for `prop:`-filtered queries — but does not add a cap, so this ticket still
stands.

Rebase onto that work rather than against current develop. Re-check whether it
has merged first.

Related: `rela-query-paging` (TKT-JU3S5N) may supply the bounding primitive.

## Acceptance

- A structured `type:` query cannot materialize an unbounded slice.
- The bound is shared with / consistent with `maxFreeTextSearchResults` rather
than a second independent constant.
- Truncation is visible to the caller (not a silent drop) — mirror whatever the
free-text path reports.
