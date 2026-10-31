---
id: TKT-0VBQW0H
type: ticket
title: Card wall board with optional drag-to-group
kind: enhancement
priority: medium
effort: l
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

## Description

Build the card-grouping prototype (branch `proto/card-grouping`) properly. It
maps steps 5 (pre-mortem) and 6 (clustering) of the `dpia-sessie.html` mockup
onto a kanban board:

- A wall layout: cards shown as a wrapping grid instead of columns.
- Note-style cards.
- Drag a card onto another card to group them. A group is an entity related to each card. Dropping on a loose card creates a new group entity.
- Inline rename of a group.
- Animations: cards slide to their new place, a lifted drag preview, a wobble on the group target.

## Requirements

1. Grouping is optional. A plain wall of orderable cards without grouping is a supported configuration.
2. Groups are more card-like. Groups can be reordered by drag, and they look visually distinct, for example a card with a different colour.
