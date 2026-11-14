---
id: TKT-K7EM7Z5
type: ticket
title: Gantt read grows faster than the data on postgres
kind: enhancement
priority: medium
effort: s
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

The postgres scale run (TKT-UHKZKOB) measured `/api/v1/_gantts/delivery` on the
perf project. From scale 1 (19k entities) to scale 5 (96k), p50 went from 344 ms
to 3.8 s (11x for 5x the data) and the statement count from 50 to 229. The read
should take a bounded number of statements and grow at most with the rows it
shows.

`tools/scalerun/scalerun.yaml` sets `ratio_max: 16` for the gantt endpoint until
this is fixed; lower it to the default when it is.
