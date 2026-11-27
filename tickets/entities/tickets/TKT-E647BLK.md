---
id: TKT-E647BLK
type: ticket
title: Range predicates on date properties in pushdown and derived indexes
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Push range predicates (`gte`/`lte`, and `gt`/`lt`) on date and datetime
properties to the store, and take them into derived-index specs. Part of the
smart-lists feature (RES-7SH71TW). Range filters such as Today and Upcoming need
them.

Date and datetime are already string-shaped (`metamodel.StringShaped`), so
`queryplan.listIndexSpec` accepts them for sort keys and `=` filters. The gap is
the operators: `listIndexSpec` takes only `=` filters, and `planListPushdown`
(`internal/dataentry/listpushdown.go`) pushes only `eq` and `ne`.

## Scope

- Push `gte`/`lte`/`gt`/`lt` on date properties in `planListPushdown`.
- Take range filters into `listIndexSpec`, with an index expression that matches
the comparison the store query uses, so the planner can use it.
- Cover both list indexes and per-control indexes.
- `storetest.RunGraphDifferential` cases for date ranges.

## Acceptance criteria

- A `gte`/`lte` date filter on a list page uses its derived index on postgres.
- Naive and SQL graph query paths agree on date-range results.
