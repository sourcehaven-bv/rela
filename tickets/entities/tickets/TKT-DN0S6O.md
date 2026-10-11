---
id: TKT-DN0S6O
type: ticket
title: 'Gantt: horizontal scroll with Now and arrow navigation'
kind: enhancement
priority: medium
effort: m
started: "2026-10-09"
completed: "2026-10-10"
status: done
---

## Description

From Atlas TASK-HJGOB, first half (the drag half is TKT-7W9LKE). The gantt
always fits the whole tree into the screen width, so a long plan is cramped and
there is no way to jump to today or page through time.

Make the timeline scroll sideways with a fixed width per day, and add "Now" and
"‹"/"›" navigation.

## Acceptance

- A span wider than the screen scrolls sideways; the tree column and the axis
stay in place.
- "Now" scrolls today into view; the arrows scroll by one zoom unit.
- The scroll position survives a reload of the data and a zoom change.
- Docs and e2e cover the navigation.
