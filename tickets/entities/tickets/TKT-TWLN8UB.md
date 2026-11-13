---
id: TKT-TWLN8UB
type: ticket
title: 'Flows M1j: db load copies unfinished runs into sqlite'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Split from TKT-FWQZGAM. `db load` into an existing sqlite database (the
in-transaction import) refuses a source with unfinished runs, because
`sqliteflowrun` needs its own connection. Copy the runs there too.

## Acceptance

- `db load` of a source with unfinished runs into an existing sqlite
database keeps the runs, and they advance afterwards.
