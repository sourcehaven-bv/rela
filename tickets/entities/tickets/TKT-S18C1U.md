---
id: TKT-S18C1U
type: ticket
title: 'Calendar views: add day and year periods'
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

Add **day** and **year** period views to the calendar view type, completing
Calendar.app parity. Month and week ship in TKT-IG54YO; this is the deferred
remainder.

## Why deferred

Split out to keep TKT-IG54YO reviewable. Month and week carry the config schema,
the sidebar/ACL wiring, the source-merging logic and drag-to-reschedule — all
the load-bearing architecture. Day and year are additional renderings on top of
machinery that already exists by then.

Year view is genuinely distinct work, not a variant: it renders **12 mini-months
with density indicators**, not event text, so it needs its own aggregation
(events-per-day counts) and its own interaction (click a day to jump to it).

## Scope

- `day` and `year` as `default_view:` values and as period-switcher options
- Day view: a single day, timed events positioned on an hour axis, all-day
events in a separate band
- Year view: 12 mini-months, per-day density indication, click-through to
the day or month view
- Navigation (next/previous/today) works in both

## Acceptance criteria

1. `default_view: day` and `default_view: year` are accepted and render.
2. The period switcher offers all four views.
3. Day view separates all-day events from timed events on an hour axis.
4. Year view shows 12 months with per-day density; clicking a day navigates to it.
5. Drag-to-reschedule behaves consistently in day view.
6. Year view's aggregation respects the read-side ACL — a hidden entity
contributes nothing to a density count.

## Depends on

TKT-IG54YO — the config schema, view registration and drag path land there.
