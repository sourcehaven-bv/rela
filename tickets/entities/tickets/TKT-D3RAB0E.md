---
id: TKT-D3RAB0E
type: ticket
title: Page executeQuery and the remaining scopedHeaders callers
kind: enhancement
priority: medium
effort: l
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

Follow-up of TKT-F2EJQY8. After it, `scopedHeaders`
(`internal/dataentry/scopedread.go`) still scans the whole type for every caller
that does not read by ids. Its allowlist entry in
`internal/archguard/unpagedread_allowlist_test.go` stays until these move.

Request-path callers to page in the store:

- `executeQuery`'s structured branch (`visibleListByTypes`): `/_search`
without free text, the dashboard query cards, the nav status markers, next
action and the `_position` scope. Push the store-safe filters, the sort and the
caller's limit into the `GraphQuery`, and count with `store.CountMatched` where
a caller needs a total. Overlaps TKT-AIEGHU (count and breakdown cards),
TKT-8AUD1U (table cards), TKT-CKLZMWB (next action) and TKT-M58EQ1W (free-text
search); coordinate with them rather than duplicate. Nav status counts are
aggregates: gate and redact before the fold (CLAUDE.md, "Aggregates computed
over the graph gate BEFORE they fold").
- Convert the calls-only budget tests: `TestQueryBudget_SearchIsSizeIndependent`,
`TestQueryBudget_SearchLinkableIsSizeIndependent`,
`TestQueryBudget_DashboardCardsAreSizeIndependent`,
`TestQueryBudget_NavStatusIsSizeIndependent`,
`TestQueryBudget_NavStatusRuleOnlyIsSizeIndependent`.

Callers that need the whole set by design (export, the calendar feed, the gantt
node load, the list-context command payload): give them their own read function
with a `batchJob` or `bounded` allowlist entry and a reason, so `scopedHeaders`
itself no longer carries a `requestPath` entry. Then delete
`TestUnpagedReadAllowlist_ScopedHeadersIsDebt`.

## Outcome (first part)

Done in this ticket:

- `/_search` without free text is planned for the store when the store can
answer exactly (`planSearch`, `internal/dataentry/searchplan.go`): equality
filters on scalar string or declared-enum properties, and a sort on one type
over string-shaped keys. Each readable type's query is built by `scopedQuery`,
so the row gate, world and face allowlist are inside it. The page reads at most
`limit` rows; `store.CountMatched` runs only when the page is full.
- The `_position` search scope over one type uses `store.GraphPosition`.
- `scopedHeaders`' verdict switch moved into `scopedQuery`.
- pgstore's `GraphPosition` ignored `OrderSpec.Values` (enum rank) and sorted
byte-wise. It now orders its window with the page's own `orderKeySQL`.
`storetest.RunGraphPositionTests` pins it on every backend.
- The search budget tests check rows where the result does not grow with the
matches (filtered, limited); the unfiltered search without a limit stays a
calls-only pin by design.

Behaviour changes:

- `meta.total` of `/_search` counts every match, also when `limit` cuts the
page. It was the number of rows served. For free text it stays the length before
the cut, which the relevance cap bounds, so there it is a lower bound.
- A query naming the same type twice (the `type` parameter plus the same
`type:` clause in `q`) returns each row once. The Go path used to read the type
twice.
- On postgres, a list or search position with an enum sort now matches the
page order.

Left for follow-ups: nav status counts (TKT-3CHTKV7), the whole-set callers' own
read functions and the `scopedHeaders` debt test (TKT-C5H506O), and the
structured search shapes the plan declines, including the mention menu's
`sort:modified:desc` (TKT-PSBGQLY). Next action and `pick_one` are TKT-CKLZMWB;
dashboard cards are TKT-AIEGHU and TKT-8AUD1U.
