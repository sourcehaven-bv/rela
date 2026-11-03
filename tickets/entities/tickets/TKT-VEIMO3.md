---
id: TKT-VEIMO3
type: ticket
title: List export reads relation columns in one batched query
kind: refactor
priority: high
effort: s
started: "2026-10-27"
completed: "2026-10-28"
status: done
---

## Description

`GET /{plural}/_export` (list export) read the edges of every exported row with
its own store query: `gatherListPeers` called `outgoingRelations` and
`incomingRelations` once per row. On the seqtrace demo (postgres, 550 tasks) one
export issued 1,107 queries and took about 1.5 s of query time (7.1 s wall time
reported by archplan). That breaks the collection-reads rule.

## Approach

Reuse `entityReader.pageRelations`, the batched edge read the list API uses (one
`RelationQuery.EntityIDs` query for all rows), and split the edges per row in
Go. Gating, redaction, face ownership and cell order stay the same, so the
export output is unchanged. Add a `storetest.Counting` budget test that asserts
the read count is the same at 10 and 50 rows.
