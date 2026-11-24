---
id: TKT-AVNEX5T
type: ticket
title: Manual list order
kind: enhancement
priority: medium
effort: l
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Description

A list with `order: manual` lets each user drag its rows into their own order.
Membership stays computed by the list; only the order is stored. Positions live
outside the graph in `state.KV`, one ordered id list per user and list.
Positioned rows come first, then the rest in the list sort. Positions apply only
to rows that passed the row gate. A move drops positions of rows that are no
longer in the list. Grouped lists keep the stored order inside each group, and
grouped relation tabs allow dragging inside a group.
