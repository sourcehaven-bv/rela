---
id: TKT-8AUD1U
type: ticket
title: 'Dashboard table cards: push limit and sort into the query'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## What

`DashboardCard.Limit` (`internal/dataentryconfig/config.go:726`) is applied
**client-side only**, in `getTableRows` via `.slice(0, card.limit)` — after the
full result set has been serialized and transferred. A `limit: 5` table card
downloads all matching rows to display 5.

`card.sort` has the same shape: sorting happens in the browser with
`localeCompare`, so it cannot be pushed down as-authored.

## Why it isn't just "add LIMIT"

Sort must move server-side *with* the limit, or the wrong rows get truncated —
limiting before an unsorted set returns an arbitrary 5, not the top 5. So this
ticket is really "make `sort` a store-side ordering, then bound it", which is
why it is `m` not `s`.

The `localeCompare` semantics are also not free to reproduce: the store would
need a compatible collation, or the config needs to declare that ordering is
byte/natural rather than locale-aware. Decide this explicitly rather than
silently changing which rows appear.

## Sequencing — check before starting

Overlaps `rela-query-paging` (branch `tkt-ju3s5n-graphquery-paging`,
TKT-JU3S5N): keyset paging for `GraphQueryer` via a separate `GraphPageQuery`.
That commit's rationale explicitly notes paging lives off `GraphQuery` because
`GraphCount` / `MatchingIDs` must answer for the whole matched set. It also
references a follow-up TKT-YWDGZD for pushing the ACL predicate down.

Build on that rather than inventing a second bounding mechanism. Re-check
whether it has merged before starting.

## Acceptance

- A `limit: N` table card transfers O(N) rows, not O(matching entities).
- Rows displayed are the same ones as today for the configured sort, or the
ordering change is documented and deliberate.
- Cards with no `limit` keep working (bounded by whatever cap the paging work
establishes).
