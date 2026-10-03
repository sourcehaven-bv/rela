---
paths:
  - "internal/dataentry/**"
  - "internal/store/**"
  - "internal/queryplan/**"
  - "internal/search/**"
---

# Collection reads

- **Collection reads are content-free, batched per page, and paged in the store
  when they can be** (TKT-1U8XYN). A list, search, kanban or scope pipeline
  reads `store.EntityHeader` rows (`ListEntityHeaders`, `GraphQueryHeaders`) and
  never a body it will not render; a body is loaded for the served rows only, on
  `include_content=true`. Per-row lookups are the defect this rule exists to
  prevent: a page loads its edges with ONE `RelationQuery.EntityIDs` query, its
  neighbours with ONE header batch, a table section its relation columns with
  one query per (column, row type). Write authorization reuses the request's
  `acl.Request` (the membership walk runs once per operation, not once per verb
  per row). When the request's shape allows it, the list handler pushes paging,
  ordering and equality filters into `store.GraphQuery` (`listpushdown.go`) and
  takes the scoped count through `store.CountMatched`, never `GraphCount`'s
  total. A query scope joins that pushdown only when `queryplan.LowerScope`
  lowers it EXACTLY (TKT-XKCNCL): nothing re-checks a pushed scope, so a
  superset pre-filter is not enough, and a scope that does not lower keeps the
  Go path. Its traversals ride in `GraphQuery.Related`, never in
  `HasInbound`/`HasOutbound`, which belong to the ACL read gate. New read
  paths pin their cost with a `storetest.Counting` budget test
  asserting the count is the same at 10 and 50 rows. Measure on the postgres
  backend with `rela-server -verbose` (`Server-Timing`, one `request` log line
  each) against `prototypes/perf/project` seeded by `rela dev seed`.
