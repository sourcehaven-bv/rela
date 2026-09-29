---
id: TKT-7RB6NI
type: ticket
title: 'Gantt hierarchy: traverse a relation from its target to its source'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

A gantt's `hierarchy:` follows each relation from its source (parent) to its
target (child). Many schemas model containment the other way: the child points
at the parent. In the tickets project, `implements` goes from ticket to feature,
so a feature roadmap gantt (feature → its tickets) cannot be configured.

Let a hierarchy entry name a relation's inverse (for example `implementedBy`),
or add a `direction: incoming` form like calendar event fields have. The
parent/child edge is then read with the endpoints swapped.

The alternative, adding a second relation that points from feature to ticket,
duplicates `implements` and is not wanted.

## Acceptance

- A gantt with `hierarchy: [implementedBy]` over `feature` and `ticket` renders each feature with its tickets as children.
- Validation rejects an inverse name that no relation declares.
- ACL row-gating and roll-up order are unchanged (gate before fold).
