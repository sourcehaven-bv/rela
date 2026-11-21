---
id: TKT-EV8U8T4
type: ticket
title: 'Record types: MCP tools, Lua list and aggregate, and CLI'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Follow-up to TKT-J7Q9C4K, which shipped the record store core and the REST
surface under `/api/v1/_records`. TKT-RK810V7 added `rela.append_records`.

This ticket adds the remaining agent and script surfaces:

- MCP tools for append, list and aggregate.
- Lua bindings for list and aggregate, next to `rela.append_records`.
- CLI: `rela records append|list|aggregate|expire`. Decide who may run
`expire`; retention today runs only as the hourly job.

The other parts of the original ticket are split out: rollups over records, the
native SQL aggregator, per-type permissions, the `entity` field type, the
backend registry, the entity-page panel, and the append audit. BUG-WOFSBFM
covers refusing to delete a ledger owner.

## Acceptance

- Each surface goes through `records.Service`, never `records.Store`.
- MCP and CLI reads are gated like the REST reads: a hidden owner's rows do
not appear and are not folded.
