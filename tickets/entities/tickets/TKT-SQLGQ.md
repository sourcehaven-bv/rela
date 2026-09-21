---
id: TKT-SQLGQ
type: ticket
title: 'sqlitestore: push GraphQuery into SQL via a shared dialect-parameterised lowering'
kind: refactor
priority: low
effort: l
tags:
- tech-debt
status: backlog
---

## Description

`sqlitestore` delegates `GraphQuery` / `GraphCount` / `MatchingIDs` to
`graphquerynaive` (iterate-and-filter in Go). That was a deliberate first step
— the naive implementation is the behavioural reference every backend is
verified against, so delegating guaranteed agreement from day one — and the
file says so. This ticket is the "later optimization" it names.

### Why now (the traversal case)

`RelationPredicate.EndpointMatch` (TKT-RELTRV) resolves the far side of a
relation through `matchesEndpoint`, which issues one `ListEntities` lookup per
candidate edge. On sqlite each of those is a real `QueryContext` round-trip,
so a traversal costs N×M statements where postgres costs one.

The cost is **not** alarming: lookups are by exact id against
`PRIMARY KEY (id, face)`, in-process, against an embedded file — microseconds,
no network, and bounded by `graphquerynaive.DepthCap`. This is a low-priority
efficiency ticket, not a defect.

### Scope is wider than traversal — deliberately

`sqlitestore`'s `EntityQuery` has **no property filtering at all**
(`entityWhere` handles type/id/face/paging only), so `GraphQuery.Props` does
not push down either. Doing traversal alone would be building the roof before
the walls. The unit of work is "sqlite pushes down GraphQuery", with traversal
as one part of it.

### Proposed shape

Do NOT hoist `pgstore/graphquery.go` wholesale. It is 832 lines / 23 functions
and most of it is postgres-shaped (world resolution, face chains, keyset
paging, the `Any` branch CTEs); sharing all of that would make every future
postgres change a two-backend change.

Instead: a small dialect interface consumed by a shared predicate-tree walker.

```go
type dialect interface {
    PropText(alias, prop string) string   // ->>            vs json_extract
    PropJSON(alias, prop string) string
    TypeOf(expr string) string            // jsonb_typeof   vs json_type
    InList(col string, vals []string) (sql string, args []any)
    ArrayContains(jsn, val string) string // ?              vs json_each
}
```

The shared half is the tree walk — `EXISTS` composition, `Negate`, the nesting
bound, the refusal of a nested inheritance expansion. Those four are exactly
what the TKT-RELTRV review caught wrong in ONE backend; having them exist once
is the real prize, ahead of the performance win.

### Dialect differences (all leaf substitutions)

| Concern | postgres | sqlite |
|---|---|---|
| Storage | `jsonb` column | `TEXT` holding JSON today (see JSONB below) |
| Extract text | `properties ->> 'k'` | `json_extract(properties, '$.k')` |
| Type test | `jsonb_typeof(x)` | `json_type(x)` |
| Array membership | `jsn ? 'v'` | `EXISTS (SELECT 1 FROM json_each(...))` |
| Array length | `jsonb_array_length` | `json_array_length` |
| List binding | `= ANY($1::text[])` | no array type → `IN (?,?,?)` |
| Byte order | `COLLATE "C"` | `BINARY` (already default) |

SQLite has supported recursive CTEs since 3.8.3, so the inheritance closures
port directly.

### SQLite has JSONB too, and it is worth considering here

Since **3.45.0 (2024-01-15)** SQLite can store its internal JSON parse tree on
disk as a BLOB, in a format it also calls "JSONB". Verified available on this
project's driver (`modernc.org/sqlite v1.58.0`, runtime `sqlite_version()`
= **3.53.4**): `jsonb()`, and `json_extract` / `json_type` /
`json_array_length` all accept a JSONB argument.

Note the name is shared with PostgreSQL but the formats are NOT compatible and
the ergonomics differ: SQLite keeps ONE set of `json_*` functions that accept
either text or JSONB, rather than PostgreSQL's separate `json` / `jsonb` types
and operators. So adopting it changes the STORAGE representation, not the
query surface — the dialect table above is unaffected.

The relevance is cost, and it is the same argument that makes this ticket
worth doing at all. `sqlitestore` currently stores properties as TEXT
(`jsonprops.go`), so every `json_extract` re-parses the document on each
access; a JSONB column is pre-parsed. That matters most for exactly the
predicate-heavy queries this ticket pushes down.

Treat it as a SEPARATE, sequenced decision rather than folding it in:

- It is a **storage migration** (rewrite the `properties` column), so it needs
  a migration step and touches the write path — whereas the lowering is
  read-side only.
- The pushdown is worth doing regardless of the representation, and the
  representation can change later without redoing the lowering.
- Measure first. TEXT-vs-JSONB is a constant-factor parse win; N×M round-trips
  → 1 is an order-of-magnitude win. Do the big one first, then see whether the
  small one still registers.

### Constraints

- **Reuse `graphquerynaive.DepthCap`** so both paths bound traversal
  identically (the sqlitestore doc comment already requires this).
- **Pass the existing `storetest` conformance suite unchanged**, including the
  `EndpointMatch` cases — nested inheritance refusal, negated chained hop,
  excessive nesting.
- **Needs an index story.** The postgres traversal index is PARTIAL
  (`WHERE type=… AND jsonb_typeof(…)='string'`) and is only matched when the
  query implies that predicate — the finding behind RR-TRV06. SQLite supports
  partial and expression indexes, but `queryplan.TraversalIndexSpecs` emits a
  spec whose DDL lives in `pgstore/derivedschema.go`. Without a shared index
  path sqlite gets correct-but-unindexed SQL, which is the exact trap that
  cost a review finding.
- **`InList` makes the arg builder dialect-aware**, not just the expression
  emitters — sqlite has no array parameter, so N placeholders. That reaches
  `sqlBuilder`, which threads through every call site.

### Risk to weigh before starting

While sqlite delegates to `graphquerynaive`, that delegation is live proof the
conformance suite is meaningful. Once sqlite has its own lowering there are
THREE implementations to hold in agreement instead of two.
