---
id: TKT-ZOU5BCT
type: ticket
title: 'Flows: fire due runs of evicted tenants'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

The flow worker (TKT-FWQZGAM) runs per resident tenant in multi-tenant
rela-server, like the realtime timers. A tenant that is not loaded (the registry
keeps at most `DefaultMaxResident` tenants) has no worker, so its timers do not
fire and its failed steps are not tried again until someone opens the tenant.

Add a cheap control-plane sweep: for each tenant schema, read the earliest
`next_due_at` of its unfinished runs, and open a tenant whose runs are due long
enough for one worker tick. It must respect the resident budget, and it must not
open tenants in a loop when a run keeps failing.

The same gap applies to the realtime timers of an evicted tenant; decide whether
one sweep serves both.

## Acceptance

- On postgres in tenant mode, a timer of a tenant that nobody opens fires
within a bounded delay.
- The sweep reads at most one row per tenant per pass and never exceeds the
resident budget.
