---
id: TKT-M58EQ1W
type: ticket
title: Search time grows linearly with the data on postgres
kind: enhancement
priority: medium
status: backlog
---

## Description

The postgres scale run (TKT-UHKZKOB) measured
`/api/v1/_search?q=<word>&limit=20`. From scale 1 (19k entities) to scale 5
(96k), p50 went from 520 ms to 2.9 s (5.6x) with a constant statement count, so
the text match scans every row despite the limit.

`tools/scalerun/scalerun.yaml` sets `ratio_max: 8` for search until this is
fixed.
