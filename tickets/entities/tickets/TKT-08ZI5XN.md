---
id: TKT-08ZI5XN
type: ticket
title: Read-time today and now in every query surface
kind: enhancement
priority: high
effort: m
started: "2026-11-16"
completed: "2026-11-17"
status: done
---

## Description

GRC needs values that depend on the current date: "evidence expired", "test
overdue", "access valid until". A stored computed property captures the time it
was written and goes stale. This ticket makes `today` (a date) and `now` (a
datetime) usable at read time on every query surface, so the comparison is made
when the data is read.

Scope:

- Surfaces: list `condition:`, query scopes, dashboard `query:` cards, search filters, the `--filter`/`--where` DSL, and predicate where it serves reads. All read paths must agree.
- Fix: `$today` is ignored in a dashboard `query:` and in search.
- One value of `today`/`now` per request, captured at the request edge, so it is a constant and can be pushed down to the store (`queryplan.LowerCondition`, pgstore/sqlitestore SQL). Lowering must stay exact.
- Comparisons: date vs `today`, datetime vs `now`, date vs datetime with a documented rule (UTC).
- Docs: fix wrong examples (`due < today`, reversed `days_between`).

Out of scope: derived properties computed on read; expiring role grants (later
work builds on `now`).
