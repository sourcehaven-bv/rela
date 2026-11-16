---
id: TKT-J7Q9C4K
type: ticket
title: 'Record types: pluggable append-only record store for time series and ledgers'
kind: enhancement
priority: medium
effort: l
started: "2026-11-13"
completed: "2026-11-14"
status: done
---

## Description

Add **record types**: append-only, typed rows attached to an owner entity and
stored outside the graph. They cover the smaller time-series and ledger-style
needs of applications built on rela (meter readings, hour bookings, KPI values,
stock or points movements, a simple cash book). This is option B of RES-BB8A86O.

The record store is a **pluggable service**. rela ships default backends, and a
specialized backend (TimescaleDB, or a separate service) can be added later
without changing the schema, the API or the ACL code.

Out of scope: reports, tax logic, currency conversion, per-record ACL,
field-level `visible:` on records, relations or search on records.

## Design

### Schema

A new top-level `records:` section in `schema.yaml`:

```yaml
records:
  meter-reading:
    owner: asset              # entity type the rows attach to
    fields:
      value: { type: decimal, scale: 3, required: true }
      unit:  { type: enum, values: [kWh, m3] }
    retention: 5y             # optional
  posting:
    owner: account
    fields:
      amount:      { type: decimal, scale: 2, required: true }
      currency:    { type: enum, values: [EUR] }
      transaction: { type: string, required: true }
      memo:        { type: string }
    ledger:
      amount: amount
      group_by: transaction   # rows sharing this value must sum to zero
      commodity: currency     # optional: zero-sum per value of this field
      closed_before: lock_date  # optional: a date property on the owner
```

- Fields reuse the existing property types, plus an `entity` ref field
(`target:` type) for dimensions such as a person or a project.
- Every row has a server-assigned id, a `ts` (caller may set it; defaults to
now) and a `recorded_at`.
- Rows are never updated or deleted, except by retention or owner deletion.

### Pluggable store

`internal/records` holds the value types, the field checks and the `Store`
contract. The contract is written so a backend does not need access to the
entity store:

- `Append(ctx, type, []Record)`: atomic per call. Batches are the normal case.
- `List(ctx, type, owner, range, cursor)`: ordered by `ts`, opaque cursor.
- `Aggregate(ctx, type, owners []entity.Ref, range, fields, funcs, bucket)`:
count, sum, min, max, avg, latest, with an optional time bucket. The caller
passes an **already gated owner list** (paged), never a graph query, so a
backend that cannot see the entity store can still answer.
`records.AggregateGeneric` over `List` is the fallback a minimal backend can
use, like `store.RollupGeneric`.
- `RekeyOwner`, `TombstoneOwner`, `Expire(type, before)`: the owner lifecycle
and retention, following channels.

Selection follows `buildChannels`: a recipe override, then the store's own
database backend, then the file backend. In addition, `.rela/records.yaml` may
name a backend (`backend: <name>`), resolved from a list of named factories in
`internal/appbuild`. That is the extension point for a specialized backend: a
new package plus one factory entry. An out-of-process backend fits the same
contract through an adapter package; none ships in this ticket.

A backend may implement optional capability interfaces (for example native time
buckets). The service checks for them at wiring and falls back to the generic
path otherwise.

Default backends:

- `sqlrecords` over `internal/sqlexec`, shared by postgres (tenant schema) and
sqlite (`rela.db`). Records are project content, so on sqlite they live in
`rela.db`.
- `filerecords`: JSONL per owner per month under the project. Simple and
correct for one local user; not a design driver (see CLAUDE.md).
- `recordstest.RunAll` conformance suite that every backend passes.

### Service, ACL and write path

A `records.Service` sits on top of `Store`; transports never call `Store`
directly.

- Read: a row is readable when its owner row is readable (the `lease.Gate`
pattern over the visibility reader), plus an optional `records:read` permission
per record type.
- Append: the owner must pass the write gate, plus an optional `records:append`
permission. Field values are type-checked against the schema.
- Aggregates gate the owner set first, then fold (the gantt rule). Results are
per principal and never cached across principals.
- No audit line per row, no search, no versioning, no automations. Appends are
attributed (`principal.Principal`) on the row itself.

### Ledger constraint

For a record type with `ledger:`:

- `Append` rejects a batch whose rows, grouped by `group_by` (and `commodity`),
do not sum to zero, with exact decimal arithmetic.
- `Reverse(transaction)` appends the negated rows with a link to the original.
- With `closed_before`, a row dated on or before the owner's lock date is
rejected.
- A balance is the `sum` aggregate per owner.

### Surfaces

- REST under the data-entry API: append (batch), list, aggregate per record
type.
- MCP tools and Lua bindings for the same three operations; CLI
`rela records append|list|aggregate`.
- Rollups: a rollup column may read a record type (`latest`, `sum`, `avg` over
a window). This needs decimal `sum` and `avg` in rollups generally.
- SPA: a records panel on the entity page with a table and a simple line chart.

### Delivery slices

1. Records core: schema, `Store` contract, both default backends, conformance
suite, service with ACL, append and list over REST, MCP, Lua and CLI, owner
rename and delete, retention.
2. Aggregates: bucketed aggregates, rollup source, decimal sum and avg, SPA
panel and chart.
3. Ledger constraint: balanced batch, reverse, period lock.

Each slice can be its own PR; split into child tickets at planning if that
helps.

## Acceptance

- `recordstest.RunAll` passes for `sqlrecords` (postgres and sqlite) and
`filerecords`, including batch atomicity, ordering, cursors, aggregates with
buckets, rekey, tombstone and retention.
- A test backend registered by name in `.rela/records.yaml` is selected at
wiring, proving the extension point.
- A principal who cannot read the owner gets no rows and no aggregate
contribution from it (`TestRecords_AggregateExcludesHiddenOwner`).
- An unbalanced ledger batch is rejected as a whole; a reversal nets to zero;
a row before the lock date is rejected.
- A rollup over a record type shows `latest` and `sum` on the owner in views.
- `just arch-lint` passes: `internal/records` does not import `internal/store`
implementations; backends do not import each other.

## Open questions

- Should appending a record be able to start a flow (threshold alerts)? If so,
through which event.
- Does remote sync carry records?
- Name: `records` vs `streams`.
