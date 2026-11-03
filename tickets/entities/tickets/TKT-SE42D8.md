---
id: TKT-SE42D8
type: ticket
title: 'Roll-ups: order datetimes chronologically in min/max/latest_by'
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

Roll-up min, max and latest_by order values as text. That is time order only for
dates in the default YYYY-MM-DD format, so TKT-6I2OPG refuses datetime
properties and custom date formats there. A stored datetime may carry an offset,
a fraction, or be a bare date, so text order is not instant order.

To lift the restriction, either normalise datetimes on every write path (UTC,
fixed-width fraction) and migrate existing values, or compare parsed instants in
each backend (Go fold, pgstore, sqlitestore) with a conformance case using mixed
offsets.
