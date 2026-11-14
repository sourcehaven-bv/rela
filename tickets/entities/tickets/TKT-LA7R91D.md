---
id: TKT-LA7R91D
type: ticket
title: Pin windowed query reads against full reads on sqlite and postgres
kind: chore
priority: low
status: backlog
---

## Description

`TestFirstN_SameLeadingRowsAsAFullRead` (TKT-CKLZMWB) checks that a read of the
first n results of a data-entry query returns the first n rows of the full read,
in the same order. It runs on memstore only. Run the same comparison on
sqlitestore and pgstore (schema-pinned DSN), including a faced type under a
ranked world, so a backend whose unpaged and windowed reads order rows
differently fails a test.
