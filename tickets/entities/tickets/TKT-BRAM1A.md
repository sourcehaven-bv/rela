---
id: TKT-BRAM1A
type: ticket
title: 'Matrix card: full integer axis range, axis titles and order; schema min/max on integers'
kind: enhancement
priority: high
effort: l
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

## Description

A dashboard `display: matrix` card with integer axes shows only the values
present in the data. A 1-5 risk matrix (`kans` x `impact`) drops rows and
columns nobody uses, and a missing middle value leaves a hole in the grid. The
card also does not name its axes.

Scope (decided with the user, option B):

1. Schema `min`/`max` on integer properties (`impact: {type: integer, min: 1, max: 5}`).
Enforced on write through the validation path, reported by `analyze properties`,
exposed on `_schema`, set on the form number input. `min > max` and min/max on a
non-numeric type are rejected at schema load.
2. Matrix axis range: an integer axis with declared min/max shows the full range,
empty cells included. Without min/max, fill every integer between the lowest and
highest observed value. Out-of-range values keep the "not placed" count. The cap
of 50 values stays.
3. Axis titles: always shown; default to the property label (or name);
`rows_label:` / `columns_label:` override.
4. Orientation: optional `rows_order: desc` (and `columns_order`), default `asc`.
5. Docs in `docs-project/` for the metamodel and the data-entry guide.
