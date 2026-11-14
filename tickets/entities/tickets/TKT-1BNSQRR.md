---
id: TKT-1BNSQRR
type: ticket
title: Postgres search still grows about 3x from scale 1 to scale 5
kind: enhancement
priority: low
status: backlog
---

## Description

Follow-up to TKT-M58EQ1W. After that change, `/api/v1/_search?q=Rotate&limit=20`
takes about 17-20 ms at scale 1 and 45-60 ms at scale 5 on the perf seed
(before: 0.5 s and 2.8 s). Two things still grow with the data:

- The GiST KNN scan on `search_title` reads most of the index before it
returns the first row: about 3,100 buffers at 96k entities. The internal pages'
trigram signatures are nearly full, so the distance bound prunes little. A
larger `siglen` (64, 256) did not change the buffers read.
- For a word whose frequency PostgreSQL underestimates (seen with "alma": 193
rows estimated, 4,719 actual), the planner picks the bitmap scan on
`search_text` and sorts all matches: about 110 ms at scale 1.

Neither is a regression, but the scale run's search ratio stays at about 3
rather than 1.

## Approaches to explore

- Rank by word similarity (`<<->`) instead of whole-title similarity, which
may prune better and rank a title containing the word first.
- Raise the statistics target on `search_text`, or extended statistics, so the
planner sees mid-frequency words.
- Related: TKT-Q918DFR and TKT-D3RAB0E in the same scaling arc. Check them
for overlap before starting.
