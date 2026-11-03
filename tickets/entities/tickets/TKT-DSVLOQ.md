---
id: TKT-DSVLOQ
type: ticket
title: Create menu puts the current page's type first
kind: enhancement
priority: medium
effort: s
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

## Description

The space Create menu lists the space's `create:` types in config order on every
page that is not an entity page. In a CRM space with `create: [opportunity,
organisatie, contactpersoon]`, a user on the Organisaties list expects
Organisatie first, and on the pipeline board Deal (opportunity) first.

On a page that shows entities of known types, the menu now puts those types
first. The other types follow in config order. This is default behaviour; there
is no config key.

## Behaviour per page

- List (`/list/<id>`): the list's entity type.
- Kanban (`/kanban/<id>`): the board's entity type.
- Calendar (`/calendar/<id>`): the entity types of its sources.
- Gantt (`/gantt/<id>`): the entity types of its sources.
- Page tab (`/p/<page>/<tab>` and `/p/<page>/<entity>/<tab>`): the types of the
view the open tab shows, by the rules above. On an entity page the menu still
offers only the types it can link (TKT-XPFXMG); the order then follows the same
rule.
- Every other page (dashboard, document, entity, search, ...): config order.

When a view shows several types, the offered ones go first in config order, not
in the view's own order. A gantt's sources are a map with no order, so the
view's order is not meaningful there.

A separator (`RlMenuSeparator`) follows the leading group when other types
follow it.

## Acceptance criteria

- On a list or kanban page, the view's type is first when offered; the rest
keep config order.
- On a calendar or gantt page with several source types, those offered types
come first in config order.
- On a page tab, the open tab's view decides the order.
- On a page without view types, the order is the config order.
- A separator follows the leading group only when other rows follow it.
- docs/data-entry.md § Spaces describes the order.
