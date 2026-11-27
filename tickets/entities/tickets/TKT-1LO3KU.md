---
id: TKT-1LO3KU
type: ticket
title: Declarative date triggers in automations (date_reached)
kind: enhancement
priority: medium
status: backlog
---

## Description

Triggers in `flows.yaml` fire only on writes. Flow timers (`after:`) count from when a run entered its state or started, never from a date property.

Add a declarative trigger that fires when an entity's date property passes, for example a review date. The scheduler runs it with an idempotency key per entity and date, so a delayed run does not fire twice. Side effects go on `jobs.Queue`.

## Original description (2026-11-27: superseded by the text above)

Out of scope for the G3 dashboard work (from the G2 brief). Automations fire on
writes only. Add a declarative `date_reached:` trigger that fires when a date
property of an entity passes (for example a review date)\, run by the scheduler
with an idempotency key per entity and date so a delayed run does not fire
twice. Side effects stay on jobs.Queue.
