---
id: TKT-7W9LKE
type: ticket
title: 'Gantt: drag bars to move and resize'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Second half of Atlas TASK-HJGOB, split from TKT-DN0S6O at the user's request:
drag a gantt bar to move it, or drag an edge to change its start or end.

The approved design is in PLAN-ND72GY (TKT-DN0S6O): a lazy, cached hover/focus
GET decides whether handles show; the commit re-reads the entity and sends one
PATCH with field preconditions; slider-pattern keyboard support; revert with a
message on 403/412/422. The server adds `face` to gantt nodes.

## Acceptance

See PLAN-ND72GY acceptance criteria 1-4 and 6.
