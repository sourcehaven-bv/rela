---
id: TKT-JDJ9VHR
type: ticket
title: Plan large id arrays per call on the remaining pgstore reads
kind: enhancement
priority: low
status: backlog
---

## Description

TKT-K7EM7Z5 made `ListRelations`, `ListEntities` and `ListEntityHeaders` run a
statement with an id array over 500 elements as an unnamed statement (`pagePlan`
in `internal/store/pgstore/pagedseq.go`). PostgreSQL then plans it for its
arguments. A cached statement may switch to a generic plan after five runs, and
a generic plan compares every row with every element of `= ANY($n)`. On the
gantt edge read that cost 8.4 s instead of 33 ms.

Other pgstore statements take id arrays and still run cached:

- graph query `Endpoints` and `MatchingFaces`
- `ListRelationsPage`
- `CountRelations`
- the revealed-relations read

Measure each with a large array on the perf project at scale 5. Where a generic
plan is slow, route the statement through `pagePlan`. Add a test per path that
fails without it, like `TestListRelations_LargeIDBatchIsNotPrepared`.
