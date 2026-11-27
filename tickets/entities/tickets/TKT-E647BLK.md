---
id: TKT-E647BLK
type: ticket
title: Date properties in derived index specs
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Accept date and datetime properties in derived-index specs. Part of the
smart-lists feature (RES-7SH71TW). Range filters such as Today and Upcoming need
them.

The derivation accepts only string-shaped properties (`metamodel.StringShaped`,
used by `queryplan.listIndexSpec`).

## Scope

- Index date properties with an expression that matches the comparison the
store query uses, so the planner can use it.
- Cover both list indexes and per-control indexes.
- `storetest.RunGraphDifferential` cases for date ranges.

## Acceptance criteria

- A `gte`/`lte` date filter on a list page uses its derived index on postgres.
- Naive and SQL graph query paths agree on date-range results.
