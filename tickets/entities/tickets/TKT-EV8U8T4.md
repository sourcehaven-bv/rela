---
id: TKT-EV8U8T4
type: ticket
title: 'Record types: MCP, Lua, CLI, rollups, SPA and backend registry'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Follow-up to TKT-J7Q9C4K, which shipped the record store core: the `records:`
schema section, the `Store` contract with file, sqlite and postgres backends,
the conformance suite, the service with owner-based ACL, bucketed aggregates,
the ledger constraint (balance, reverse, period lock), owner rename and delete,
retention, and the REST surface under `/api/v1/_records`.

This ticket covers the parts TKT-J7Q9C4K deferred:

- MCP tools and Lua bindings (`rela.records`) for append, list and aggregate.
- CLI: `rela records append|list|aggregate|expire`, and running `Expire` on a
schedule.
- Rollup source: a rollup column reads a record type (`latest`, `sum`, `avg`
over a window), with decimal `sum` and `avg` in rollups generally.
- SPA: a records panel on the entity page with a table and a line chart.
- Named backend registry: `.rela/records.yaml` `backend: <name>` resolved from
factories in `internal/appbuild`, with a test backend proving selection.
- Optional `records:read` and `records:append` permissions per record type.
- An `entity` ref field type (`target:`) for dimensions such as a person.

Review items deferred from TKT-J7Q9C4K:

- A native SQL `Aggregator` for `sqlrecords` (bucket and fold in the
database), so aggregates are not capped at `MaxAggregateRows`, with a
`storetest.Counting`-style budget test.
- Drop an owner's rows inside the entity delete transaction on postgres and
sqlite, or tombstone the id, so a failed hook never leaves rows a reused id
would inherit.
- Read the ledger lock date inside the append transaction, so a period
closed while a posting is in flight stays closed.
- Refuse to delete an owner that still has ledger rows.
- Decide whether record writes go to the audit log.

## Acceptance

- Each surface goes through `records.Service`, never `records.Store`.
- A test backend named in `.rela/records.yaml` is selected at wiring.
- A rollup over a record type shows `latest` and `sum` on the owner in views,
gated before it folds.
