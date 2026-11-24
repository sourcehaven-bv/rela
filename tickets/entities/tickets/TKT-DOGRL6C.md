---
id: TKT-DOGRL6C
type: ticket
title: Calendar date buckets for grouped lists (day/week/month/year, near-then-far, newest first)
kind: enhancement
priority: medium
effort: m
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Description

A list grouped on a date can only use `buckets: relative`. Add calendar buckets:
`buckets: { by: day|week|month|year }`, a near-then-far form `{ by: day, for:
7d, then: month }`, and `order: desc` for newest first. Buckets follow the
reader display time zone, as relative buckets do.
