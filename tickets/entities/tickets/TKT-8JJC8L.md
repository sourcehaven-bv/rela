---
id: TKT-8JJC8L
type: ticket
title: Configure editor for list, matrix and ratio dashboard cards
kind: enhancement
priority: low
status: backlog
---

## Description

The Configure space's Dashboard screen edits query cards only. TKT-2SGPJZ added
`list:` cards and the `matrix` and `ratio` displays, with the keys `list`,
`rows`, `columns` (as a property name), `tone_by`, `colors` and `match`. Those
keys are locked in `internal/configedit/allowlist.go`, so an operator writes
them in `data-entry.yaml`.

Add them to the Configure editor:

- choose a list instead of a query;
- for a matrix, pick the row and column properties, `tone_by` and a colour
per value;
- for a ratio, pick the `match:` query scope and an optional `group_by`;
- unlock the keys in the allowlist.

Out of scope: changes to how cards are computed.
