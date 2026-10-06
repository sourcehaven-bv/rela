---
id: TKT-65LVAK
type: ticket
title: Enforce relation cardinality at write time and add an atomic replace operation
kind: enhancement
priority: high
effort: m
status: backlog
---

## Description

Enforce `max_outgoing` and `max_incoming` at write time, and add an atomic
`replace` operation for single-valued relations. Part of RES-8CKUNJ
(relation-backed status); prerequisite for lookup properties and for kanban
columns from a relation.

Today `max_outgoing` is analysis-only (`internal/schema/cardinality.go:216`). A
relations PATCH applies `add`/`remove` deltas edge by edge, and a write-loop
error leaves earlier edges written (`relations_modern.go:336`). Re-pointing a
task from one status to another is therefore two writes that can leave zero or
two edges.

## Scope

- Reject a write that would exceed `max_outgoing` or `max_incoming`, on every
write path (data-entry API, MCP, CLI, Lua). The error names the relation and the
limit.
- Add a `replace` operation to the relations PATCH for relations with
`max_outgoing: 1`: remove the existing edge and add the new one in one store
transaction.
- Decide whether existing data that already exceeds a limit is grandfathered
(warning on read, error only on the next write that adds an edge).

## Acceptance criteria

- A second `add` on a `max_outgoing: 1` relation is rejected with a clear
error on all write paths.
- `replace` leaves exactly one edge, including when the old edge is absent.
- A failed `replace` leaves the original edge in place.
