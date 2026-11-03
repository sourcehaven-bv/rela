---
id: TKT-8Z2LT3
type: ticket
title: Show a related entity's property on a kanban card
kind: enhancement
priority: medium
effort: m
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

## Description

A kanban card cannot show a property of a related entity. Sourcehaven's atlas
CRM pipeline board (entity `opportunity`) should show the segment of the linked
organisation on each card as a coloured badge, such as "Ziekenhuis / UMC". The
organisation links to the opportunity through `heeft_opportunity` (organisatie
to opportunity), so seen from the card the relation is incoming.

Today a card field `{relation, direction}` shows the titles of the related
entities. There is no way to show one of their properties instead.

Related: TKT-DA9C0L (Relation paths) is the larger backlog item. It adds a
`<relation>.<property>` path for single-valued, outgoing relations across all
view config. This ticket is smaller: one hop, kanban cards only, both
directions, any number of related entities.

## Scope

In scope:

- Kanban card field `{relation, direction, property, label?, show_label?}`.
It shows the value of `property` on each related entity, deduplicated, and
renders it as that property normally renders: enum values as badges styled by
the property's `styles:`, integers with `display: {currency}` as amounts, dates
formatted.
- Data comes from the existing `include=*` board fetch; the backend does not
change. A related property the reader may not see (`visible:`) is stripped from
the include block, so the card shows nothing for it. A Go test pins it.
- Config validation: the relation exists and the board's entity type is on the
correct side for the direction; `property` exists on every type at the other
end; the form is refused with `display: count`, on calendar event fields and on
gantt tooltip fields.
- Docs, Go tests, Vitest and an e2e test.

Out of scope: multi-hop paths, the form on calendars and gantts, sorting or
filtering by a related property (TKT-DA9C0L).

## Acceptance criteria

1. A card field `{relation: heeft_opportunity, direction: incoming, property:
segment}` shows the segment of the linked organisation as a badge in the colour
from the enum's `styles:`.
2. Several related entities show each distinct value once.
3. An integer with `display: {currency: EUR}` on the related type renders as an
amount; a date renders formatted.
4. A related property hidden by `visible:` is absent from the include block and
the card shows nothing for the field.
5. Config load rejects: an unknown relation, a board type on the wrong side of
the relation for the direction, a property missing on one of the target types
(the error names that type), `property` with `display: count`, and the form on
calendar and gantt fields.
