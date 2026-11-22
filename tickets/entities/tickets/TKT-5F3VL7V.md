---
id: TKT-5F3VL7V
type: ticket
title: Native SQL aggregator for sqlrecords
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Split from TKT-EV8U8T4 (review item deferred from TKT-J7Q9C4K). `sqlrecords` has
no native `Aggregator`, so aggregates fall back to `AggregateGeneric` and are
capped at `MaxAggregateRows`. Bucket and fold in the database on postgres and
sqlite.

## Acceptance

- `sqlrecords` implements `records.Aggregator`; results match
`AggregateGeneric` on the conformance suite.
- A `storetest.Counting`-style budget test pins the query count.
- No `MaxAggregateRows` cap on the SQL backends.
