---
id: TKT-RK810V7
type: ticket
title: Metric capture and trend charts over records
kind: enhancement
priority: medium
effort: m
started: "2026-11-16"
completed: "2026-11-17"
status: done
---

## Description

Make periodic metric capture and trend charts possible with records (GRC gap
item 6).

1. Lua binding `rela.append_records(type, rows)` on writer runtimes only. It goes through `records.Service.Append`, the same authorization as `POST /api/v1/_records/{type}`: the caller may append only when it may update every face of the owner.
2. Document the capture pattern in the scheduled-tasks and records guides: a `metric` owner entity per series, a record type `measurement`, and a scheduled task with `run_as` that computes the value and appends one row. The metric entity's read rule must be no wider than what `run_as` can see.
3. A `display: line` dashboard card with a `records:` source that reads `/_records/{type}/_aggregate` for one or more owners, with a bucket and range. Owners the reader cannot read are not folded and do not appear.

The declarative capture without Lua is a separate ticket.
