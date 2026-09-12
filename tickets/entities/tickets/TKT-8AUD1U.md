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

## Sequencing — resolved

`store.GraphQuery.OrderBy` / `Limit` / `Offset` exist as of TKT-1U8XYN (#1526),
pushed into SQL on pgstore and into `graphquerynaive.Order` / `Page` on the
delegating backends, with paging conformance in `storetest/graphquery.go`. Build
on those. `internal/dataentry/listpushdown.go` is the worked example of pushing
a list shape down; a dashboard table card is the same problem with a smaller
limit.

This section previously pointed at TKT-JU3S5N (branch
`tkt-ju3s5n-graphquery-paging`), which proposed keyset paging on a separate
`GraphPageQuery`, and asked implementers to build on it rather than invent a
second bounding mechanism. That re-check did not happen before TKT-1U8XYN
shipped, so for a while two designs for one capability existed. TKT-JU3S5N is
now closed `wont-fix` as superseded. Noted only so the history reads clearly.

### Note on keyset vs offset, for whoever bounds these cards

Offset paging is what exists and is the right default here: it supports
arbitrary `OrderBy` and random access into the result set, which is what a list
UI with sortable columns and jump-to-page needs, and a dashboard card's `limit:
5` is the first page of an ordered set — offset's weakest case is not in play.

Keyset (cursor) paging has two advantages at a different operating point. It is
stable under concurrent writes: an offset page can drop or repeat a row when a
row earlier in the ordering is inserted or deleted between requests, whereas a
cursor resumes from a key and cannot. And it stays cheap at depth, since the
engine seeks to the key rather than counting past `Offset` rows it then
discards.

Neither advantage is free. Keyset needs a total order with a unique tiebreak
(id), and it cannot jump to an arbitrary page. So if a future consumer needs a
deep or long-lived walk — exporting every matching row, or an ACL-filtered scan
where a silently dropped row reads as a nonexistent entity — consider a cursor
for that path specifically, rather than converting the list path. TKT-JU3S5N's
review record (branch `tkt-ju3s5n-graphquery-paging`, commit `f83217ff`,
unpushed) has the prior art, including a hostile-id conformance technique for
catching ordering divergence that well-behaved test ids cannot detect.

## Acceptance

- A `limit: N` table card transfers O(N) rows, not O(matching entities).
- Rows displayed are the same ones as today for the configured sort, or the
ordering change is documented and deliberate.
- Cards with no `limit` keep working (bounded by whatever cap the paging work
establishes).
