---
id: TKT-RS31NGP
type: ticket
title: Write record audit lines in the record transaction
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Record audit lines (append, update, delete of record rows) are written after the
record write commits, best effort. Entity writes write theirs inside the write
transaction since TKT-TU0VZ4Y, so a line exists if and only if the write
committed.

Do the same for records on postgres and sqlite: `sqlrecords` runs its own
`Transact`, so the audit append needs to join that transaction (a hook on the
record store, and an audit log over the `sqlexec.Querier`). An audit error rolls
the record write back (DEC-U78WD7Y).

## Acceptance

- On postgres and sqlite, a record write and its audit line commit or roll
back together.
- The fs tier keeps the best-effort JSONL line.
