---
id: TKT-N7P2RUP
type: ticket
title: Roll-ups in lists and dashboard cards, decimal sum, total cards
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Roll-up properties (TKT-6I2OPG) appeared only on the single-entity GET. This
extends them to the places their properties appear, and adds totals.

- List rows carry every roll-up by default (`rollups=false` opts out). A list
can filter, sort and group on a roll-up; the Go path computes the queried
roll-ups for the scoped population before paging, and the pushdown declines.
- Dashboard breakdown, matrix, ratio and table cards read roll-ups.
- `sum` accepts a `decimal` property and returns an exact decimal on every
backend.
- New card displays `sum`, `avg`, `min` and `max` with `property:` fold one
property over a list card's rows, gated and redacted before the fold.
