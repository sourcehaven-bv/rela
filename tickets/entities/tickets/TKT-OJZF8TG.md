---
id: TKT-OJZF8TG
type: ticket
title: 'Data entry guide: total card min and max property types'
kind: docs
priority: low
effort: xs
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

GitHub issue #2239. The data entry guide, section "Total cards", said `min` and
`max` take "a property a list can sort on". `summaryPropertyTypes` in
`internal/dataentryconfig/summary.go` accepts only `integer`, `decimal`, `date`
and `datetime`, so a `string` or `enum` property, which a list can sort on,
fails at config load.

## Fix

Name the four accepted types in `GUIDE-data-entry.md` and regenerate
`docs/data-entry.md`. The code restriction is intended: kanban `column_summary`
shares it.
