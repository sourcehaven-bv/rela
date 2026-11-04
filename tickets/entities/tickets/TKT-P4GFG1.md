---
id: TKT-P4GFG1
type: ticket
title: Pile toggle action (favorite star) and pile navigation entries
kind: enhancement
priority: medium
effort: m
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

## Description

Add an operator-defined pile toggle, such as a favorite star, and sidebar
navigation entries for declared piles. This reproduces the "favorite" star next
to an entity title and a favorites section in the sidebar from the Atlas design.

```yaml
# data-entry.yaml
piles:
  toggles:
    favorite:
      pile: favorites          # declared pile key
      icon: star               # shown when the entity is not on the pile
      icon_active: star-fill   # shown when it is
      label: Favorite
      placement: title         # next to the entity title
      entity_types: [initiative, project]

navigation:
  - pile: notifications        # badge with unread count when read state is on
  - pile: favorites            # lists the pile's items
```

## Scope

- Toggle config with validation: the pile must be a declared pile, icons must exist in `icondefs`, entity types must exist.
- Detail page: render the toggle at its placement. One membership lookup for the entity on screen.
- Lists: if toggles are shown per row, membership is read once per page in a batch, never per row (`collection-reads.md`), pinned by a budget test.
- Toggle adds or removes the entity's served face (`ID@face`).
- `pile:` navigation entry renders a declared pile in the sidebar, outside the "Piles" group, with its items as links.
- A declared pile that no navigation entry places gives a load warning.

## Depends on

The declared-piles ticket.
