---
id: TKT-M58EQ1W
type: ticket
title: Search time grows linearly with the data on postgres
kind: enhancement
priority: medium
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

The postgres scale run (TKT-UHKZKOB) measured
`/api/v1/_search?q=<word>&limit=20`. From scale 1 (19k entities) to scale 5
(96k), p50 went from 520 ms to 2.9 s (5.6x) with a constant statement count, so
the text match scans every row despite the limit.

`tools/scalerun/scalerun.yaml` sets `ratio_max: 8` for search until this is
fixed.

## Cause

The scale-run word matches about 85% of rows. The query ranked every match by
`similarity()` on a title expression and only then applied the limit, so it read
and sorted nearly the whole table.

## Fix

- Migration 0047 adds `entities.search_title`: the lowercased title property,
or the id when there is none. A trigger keeps it current from the
`search_title_props` map, which Open records from the schema.
- A GiST trigram index on it lets search read hits in distance order
(`search_title <-> needle, id`) and stop at the limit.
- World resolution is an anti-join, so it does not break that order.
- Search statements run with per-execution plans (`QueryExecModeExec`).
- A background heal pass, guarded by an advisory lock, fills the column after
the upgrade and after the title map changes.
- `SearchVisible` stops once the limit is reached, which drops one statement.

## Measurements (p50, manager/editor/reader)

| | scale 1 | scale 5 |
| --- | --- | --- |
| Before | 493/239/475 ms | 2768/1304/2683 ms |
| After | 9/22/10 ms | 28/42/28 ms |

Search `ratio_max` tightened from 8 to 5, p95 limits from 2000/9000 ms to
100/150 ms.

## Behaviour changes

- Ranking is by title distance ascending (same order as similarity
descending). Without a title map there is no prefix ranking; rows rank by id.
- An empty title ranks as the id.
- Right after the upgrade, rows rank as untitled until the heal pass reaches
them.

## Follow-ups

- TKT-2EST91Z: a needle sharing no trigram with any title still sorts all
matches.
- TKT-1BNSQRR: search still grows about 3x from scale 1 to scale 5.
- TKT-841NYIH: the scale run measures before autoanalyze and flakes on list
reads.
- TKT-Q918DFR and TKT-D3RAB0E (same arc) are not in this branch; TKT-1BNSQRR
notes to check them for overlap.
