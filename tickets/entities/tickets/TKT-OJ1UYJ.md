---
id: TKT-OJ1UYJ
type: ticket
title: Push the relation-count endpoint gate into the store
kind: enhancement
priority: low
effort: m
tags: tech-debt
status: backlog
---

## Description

`ScriptReader.CountRelations` (TKT-QZTROQ) counts the gated relation list: it
loads every edge of the type and gates both endpoints in one batch. The remote
MCP schema overview and summary prompt do this once per relation type, so their
cost grows with the number of edges. The raw count it replaced was one
`count(*)` per type.

Push the endpoint gate into the store instead: count edges whose head and tail
rows match the caller's read scope for their types, as a join on the composed
ACL query. This is the relation twin of `countPushdown`.

Also bind once per schema call rather than once per type.

## Acceptance

- A relation count runs as one store statement per relation type on
pgstore and sqlitestore.
- It equals the length of `ListRelationsStrict` for every grant kind,
pinned by a parity test like `TestCountPushdown_EqualsListPushdown`.
