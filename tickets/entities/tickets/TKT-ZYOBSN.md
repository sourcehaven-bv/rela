---
id: TKT-ZYOBSN
type: ticket
title: Add a datetime metamodel property type (time-bearing, with date+time form widget)
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

rela's metamodel has a `date` property type but **no `datetime`/timestamp type**
(built-ins: string, date, integer, boolean, enum, file, rrule —
`internal/metamodel/types.go:249-255`). A `date` value can technically hold an
RFC3339 timestamp via a custom `Format`, but there is no distinct type and no
date+**time** form widget, so authoring a time-bearing value means hand-editing
RFC3339 into a date field — poor UX.

Add a first-class **`datetime`** property type:
- new `PropertyTypeDateTime` constant + parser (`ParseDateValue` already accepts
RFC3339; formalize a datetime path)
- validation, filter (`<`/`>` typed comparison), and sort support
- a data-entry form widget that captures **date + time** (and timezone handling)

Related prior art: `RR-9ZQLP` (separate datetime formatter, deferred),
`RR-QUXNPR` (time.Time values hit a default branch), concept `metamodel-types`.

## Why now / who needs it

**Blocks timed calendar events in the feed feature (TKT-RDM9M5 / FEAT-OT4361).**
That feature ships **all-day events only** (`date` prop → iCalendar
`VALUE=DATE`) precisely because there's no clean way to author a timed value.
Once `datetime` exists, a `datetime` prop → timed `DTSTART`, and a `datetime`
start/end pair → a timed range VEVENT — the feed picks it up with no feed-side
redesign (type-driven: `date` = all-day, `datetime` = timed).

This is a metamodel primitive, not calendar work — hence its own ticket. Not
required for the PIM nudge (tasks are due *on a day* = all-day), so low priority
until timed events are actually wanted.
