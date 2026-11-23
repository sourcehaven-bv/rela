---
id: TKT-VMSWSWP
type: ticket
title: Rollups over record types
kind: enhancement
priority: medium
effort: l
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Split from TKT-EV8U8T4. A rollup column can read a record type instead of
relations: `latest`, `sum` or `avg` of one field over a window, per owner.
Rollups also gain decimal `sum` and `avg` in general.

The rollup reads a second store on the list path, so it batches per page
(`collection-reads.md`) and gates the owner set before it folds, as the gantt
endpoint does. Run `/research` or `/design-review` first.

## Acceptance

- A rollup over a record type shows `latest` and `sum` on the owner in views.
- Reads go through `records.Service`, never `records.Store`.
- A budget test pins one record read per page, not per row.
- A row of a hidden owner never contributes to a visible value.
