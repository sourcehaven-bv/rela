---
id: TKT-CKLZMWB
type: ticket
title: Next-action suggestion grows with the data on postgres
kind: enhancement
priority: medium
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

The postgres scale run (TKT-UHKZKOB) measured `/api/v1/_next_action`. From scale
1 (19k entities) to scale 5 (96k), p50 went from 33 ms to 217 ms (6.6x) and the
statement count from 12 to 34.

`tools/scalerun/scalerun.yaml` sets `ratio_max: 9` for next-action until this is
fixed.
