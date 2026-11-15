---
id: TKT-J7R3HK6
type: ticket
title: Reconcile drops renamed query indexes before it builds the new ones
kind: enhancement
priority: low
status: backlog
---

## Description

pgstore `Reconcile` drops derived query indexes whose names are no longer
computed before it creates the desired ones. A change to the index layout
renames every query index (TKT-CKLZMWB added id as the last key column), so on
the first open with a new build each tenant drops all its query indexes and
rebuilds them with a plain `CREATE INDEX`, which blocks writes while it runs.
Queries run without the index in between. During a rolling deploy an old process
that reconciles at open drops the new indexes again.

Create the desired indexes first and drop orphans afterwards, and consider
`CREATE INDEX CONCURRENTLY` outside a transaction.
