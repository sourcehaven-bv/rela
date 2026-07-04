---
id: FEAT-27UZ51
type: feature
title: Datetime property type
summary: A first-class time-bearing metamodel property type (date + time), with a date-time form widget, so entities can carry timestamps that filters, sorts, and calendar feeds treat as timed rather than all-day.
description: 'Add a first-class time-bearing `datetime` metamodel property type: a new PropertyTypeDateTime constant + RFC3339 parser, validation, typed filter (</>) and sort support, and a data-entry form widget capturing date AND time (with timezone handling). Distinct from the existing date type (which yields a date-only picker). Downstream, the calendar-feed feature (FEAT-OT4361) renders timed events from datetime properties. Delivered by TKT-ZYOBSN.'
priority: low
status: proposed
---

rela's metamodel has a `date` type but no time-bearing `datetime` type. Adding
one (with a proper date+time form widget, filter/sort support, and RFC3339
parsing) lets entities carry real timestamps. Downstream, the calendar-feed
feature (FEAT-OT4361) can then render timed events (`DTSTART` with a time, and
start/end ranges) instead of all-day-only. Parallel in shape to FEAT-DUW9 (the
rrule field type). Delivered by TKT-ZYOBSN.
