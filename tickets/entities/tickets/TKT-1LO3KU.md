---
id: TKT-1LO3KU
type: ticket
title: Declarative date triggers in automations (date_reached)
kind: enhancement
priority: medium
status: backlog
---

## Description

Out of scope for the G3 dashboard work (from the G2 brief). Automations fire on
writes only. Add a declarative `date_reached:` trigger that fires when a date
property of an entity passes (for example a review date)\, run by the scheduler
with an idempotency key per entity and date so a delayed run does not fire
twice. Side effects stay on jobs.Queue.
