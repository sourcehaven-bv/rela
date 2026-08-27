---
id: RR-ZC4KC1
type: review-response
title: Unbounded multi-day expansion could hang the tab
finding: eventsByDay walked from an event's start to its end with no upper bound. An entity with a far-future end date produces millions of iterations and Map entries on the main thread — a hard tab hang from one typo'd year. A comment claimed the loop could not run away; it only guarded end before start.
severity: critical
resolution: Clamp each event's span to the visible grid before walking it so cost scales with the rendered cells rather than the data. Also replaced the hand-rolled cursor with the table-tested addDays.
status: addressed
---

## Finding

`eventsByDay` walked a cursor from an event's start day to its end day, defended
by the comment "a malformed end was already clamped to the start, so this cannot
run away."

That comment was wrong. The clamp only handles `end < start`; nothing bounds
`end` from above. An entity with `due: 2026-01-01` and `due_end: 9999-12-31` — a
plausible typo, or a deliberate "ongoing" marker — produces ~2.9 million
iterations and Map entries on the main thread. A hard tab hang from one bad
date, with the misleading comment discouraging anyone from looking.

The window fetch does not save you: `max_span` pads only the LOWER bound, so an
event starting inside the window with an absurd end date is fetched and then
expanded in full.

## Resolution

`eventsByDay` now takes the visible days and clamps each event's span to that
window before walking it. Cost scales with the grid (~42 cells), not with the
data, so the runaway class is removed rather than bounded by a magic number.

Also replaced the hand-rolled cursor increment with `addDays`, which already
handles month and year rollover and is table-tested.

## Note on the comment

Worth calling out separately: the defect was not just the missing bound but a
comment asserting a safety property that did not hold. A wrong comment is worse
than no comment — it answers the question a reader would otherwise ask.
