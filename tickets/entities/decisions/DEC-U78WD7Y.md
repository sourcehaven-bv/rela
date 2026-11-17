---
id: DEC-U78WD7Y
type: decision
title: Database backends fail an entity write whose audit record cannot be written
context: 'The audit log was best-effort on every backend: the record was appended after the store write, and a failed append was logged and dropped. That suits a single developer on the filesystem backend, whose project is in git. Networked deployments run on PostgreSQL or SQLite and need a complete record of who changed what (GRC gap 11). Those backends have transactions, so the record can be written in the same transaction as the change.'
consequences: On PostgreSQL and SQLite a write and its audit record commit or roll back together, which closes the crash window. A database fault in the audit tables now blocks writes; that is the intended trade. Every write takes the audit chain lock, which serializes writers per tenant schema for the length of the audit insert. The filesystem backend keeps the best-effort rule. docs/audit-log.md changes its 'never block a write' statement for the database builds when TKT-TU0VZ4Y lands.
date: "2026-11-16"
status: accepted
---

## Decision

On the PostgreSQL and SQLite backends, the audit record of an entity or relation
write is appended in the same `store.Store.Tx` as the write. If the append
fails, the write fails and rolls back.

The filesystem backend keeps the JSONL sink and its best-effort rule: a failed
audit write is logged and the entity write succeeds.

## Rollout

- TKT-USQBMG6 adds the SQL log, the hash chain and `rela audit verify/prune/import`. Records are still appended after commit by a background writer, so the rule is unchanged in that PR.
- TKT-TU0VZ4Y moves the append into the write transaction and makes it fail closed.
