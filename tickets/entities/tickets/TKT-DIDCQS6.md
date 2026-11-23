---
id: TKT-DIDCQS6
type: ticket
title: Bound the gantt and sankey source-type read
kind: chore
priority: low
effort: m
status: backlog
---

## Description

Follow-up of TKT-C5H506O. The AllowAll arm of `loadGanttType`
(`newGanttHandler`, `internal/dataentry/gantt_handler.go`) reads every readable
row of a gantt or sankey source type, then filters by `where:` and caps the
response at `MaxNodes`. Roll-ups fold dates past the cap, so a bound needs the
roll-up computed in the store (`store.Rollup`) or the `where:` pushed down
first. Keep the gate-before-fold order (TKT-MW28U5). Then remove its
`request-path` entry in `internal/archguard/unpagedread_allowlist_test.go`.
