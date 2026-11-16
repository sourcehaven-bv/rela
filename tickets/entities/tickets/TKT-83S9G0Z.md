---
id: TKT-83S9G0Z
type: ticket
title: Coloured cells on a kanban matrix
kind: enhancement
priority: medium
effort: m
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Description

A kanban board with `swimlane_property` is a matrix: columns by swimlanes, for
example impact by likelihood in a DPIA risk assessment. Let the config give each
cell a background tone, so the board reads as a risk heat map.

Add `cell_colors:` to a kanban: a map from a column value to a map from a lane
value to a colour. A colour is a tone (success, warning, error, info, new) or a
colour name from the TKT-ZKNWSK vocabulary, with an optional label. The server
validates it at load and resolves each colour to a style class. The SPA paints
the cell with theme tokens and shows the tone label as text in the cell, so
colour is not the only cue.

## Acceptance criteria

- An unknown column, an unknown lane or an unknown colour fails config load
with a message naming the board.
- `cell_colors` without `swimlane_property` or with `columns_from` fails.
- A colour that is not a tone needs a `label`.
- A coloured cell shows its label as text, in both themes.
- The cards stay readable: they keep their own background.
