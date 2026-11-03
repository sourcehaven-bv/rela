---
id: TKT-40TEOZ
type: ticket
title: Dashboard trend cards from stored count snapshots
kind: enhancement
priority: medium
status: backlog
---

## Description

Out of scope for the G3 dashboard work. A trend card shows how a count changed
over time (for example open risks per week). That needs count snapshots stored
on a schedule, because the graph only holds the current state. Decide where
snapshots live (state.KV or a tenant table), how often they are taken, and how a
card declares the series. Snapshots are per-principal only if the count is; a
shared snapshot must be taken under a system reader and shown only to principals
who may see the source list.
