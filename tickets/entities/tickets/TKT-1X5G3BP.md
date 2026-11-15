---
id: TKT-1X5G3BP
type: ticket
title: Create a column from the board
kind: enhancement
priority: medium
effort: m
status: ready
---

## Description

On a kanban board whose columns come from an enum property, a facilitator adds a
column from the board itself instead of editing `schema.yaml`. It is optional
per board (`add_column: true` in `data-entry.yaml`) and off by default.

`column_property` must be an enum, so a new column is a new enum value: a config
write. The board writes it through the Configure API, the only server-side
config writer, whose allowlist already permits
`entities.*.properties.*.values.[]`. The control only shows when the principal
may configure (`/_me.can_configure`).

Part of the DPIA workshop follow-ups to TKT-35BY3PE.
