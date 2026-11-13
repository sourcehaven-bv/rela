---
id: TKT-D3RAB0E
type: ticket
title: Page executeQuery and the remaining scopedHeaders callers
kind: enhancement
priority: medium
effort: l
status: backlog
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
