---
id: TKT-B51CYD
type: ticket
title: 'sqlitestore: push GraphQuery down into SQL like pgstore'
kind: enhancement
priority: high
effort: xl
status: backlog
---

## Description

`sqlitestore` answers `GraphQuery`, `GraphCount` and `MatchingIDs` through
`graphquerynaive` (`internal/store/sqlitestore/graphquery.go`): it loads the
candidate rows and filters them in Go. pgstore renders the same query as SQL. A
scoped or ACL-gated list on sqlite therefore costs a full read of the type,
where postgres pages in the database.

Make sqlite do what postgres does:

- Render `store.GraphQuery` as SQL: props, `HasInbound`/`HasOutbound`
(including inheritance expansion and `EndpointMatch` chains, bounded by
`graphquerynaive.DepthCap`), `Any`, `Narrowing`, `Related` (TKT-XKCNCL), world
scoping, faces, ordering, paging.
- Implement `MatchedCounter` (the scoped count) and header-only queries in SQL.
- Reconcile the derived static-query indexes from `queryplan` as sqlite
expression indexes, all-or-nothing as on postgres.

Acceptance:

- `storetest` graph-query, endpoint-match and visible-search suites pass on
sqlite through the SQL path.
- A differential test compares the SQL path with `graphquerynaive` on the
same fixture.
- `EXPLAIN QUERY PLAN` tests show the derived indexes are used, mirroring the
pg EXPLAIN tests.
- A `storetest.Counting` budget test shows the same query count at 10 and 50
rows on sqlite.

Stacked on TKT-XKCNCL.
