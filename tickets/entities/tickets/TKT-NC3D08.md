---
id: TKT-NC3D08
type: ticket
title: Support relation fields (incoming/outgoing) on kanban cards
kind: enhancement
priority: medium
effort: m
status: ready
---

## Problem

There is no way to render a relation target on a kanban card — incoming or
outgoing. `KanbanCard.Fields` is `[]ViewSectionField` (`config.go:292`), and
`ViewSectionField` (`config.go:436`) carries only `Property`. The SPA card
renderer `getCardFieldValue` (`KanbanView.vue:256`) reads
`entity.properties[field.property]` only.

Unlike gaps 1 and 2, this is genuinely net-new — not a migration regression.

## Scope

IN: `card.fields` may reference a relation with a direction; the card renders
the target titles. OUT: kanban relation **filter controls** (shares the gap-2
fix; KanbanView option population at `KanbanView.vue:205` is property-only and
would ride on TKT-5U7QBR).

## Recommended approach

1. Introduce a dedicated `KanbanCardField` type (`Property`, `Relation`, `Direction`, `Label`) rather than widening the shared `ViewSectionField` (reused by form relations, side panels, view sections — widening leaks card semantics into all of them). Change `KanbanCard.Fields` to `[]KanbanCardField`. Backward compatible: `- property: X` still parses.
2. Reuse the **same wire mechanism chosen in TKT-ODHV2D** (ID + `included` client resolution), not a card-specific title map. KanbanView currently requests **no** `?include`, so add `include=*` when any card field is a relation.
3. SPA: `getCardFieldValue`/`getCardFieldLabel` become relation- and direction-aware.

## Dependencies

Depends on TKT-ODHV2D landing the shared incoming-relation wire mechanism, so
lists and cards share one approach.

## Acceptance criteria

- `card.fields: [{property: status}, {relation: verantwoordelijk_voor, direction: incoming}]` → card shows the status label and the responsible person's name.
- Outgoing relation card field renders its target(s).
- Existing property-only card configs render unchanged (schema loads, no breakage).

## Test plan

- Config parse test: old `- property: X` fields and new relation fields both unmarshal.
- SPA unit test on `getCardFieldValue` for incoming + outgoing relation fields.
- Board render test that a relation card field shows resolved titles via `included`.
