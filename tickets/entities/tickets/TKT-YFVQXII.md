---
id: TKT-YFVQXII
type: ticket
title: Saved smart lists per user
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Let a user save a list's filter-control values as a named smart list. Part of
the smart-lists feature (RES-7SH71TW).

## Scope

- Saved state: `{list, name, control key → op and value}` per user, outside
the graph. Decide the store (new per-user store or piles); postgres keeps it in
the tenant schema.
- On load, validate the values against the list's current `filter_controls`
and their `ops:`. Drop keys the operator removed and show that to the user.
- Rows come from the gated list API under the reader's own identity. A smart
list adds no read path.
- Include the saved state in tenant export and delete it with the user.
- SPA: save, rename, delete and pick a smart list on a list page. Reuse the URL
filter state (BUG-EXJ54CD).

## Security

- A saved value may hold an entity id (relation control). It is the user's own
state and never shown to others.
- Sharing smart lists is out of scope.

## Acceptance criteria

- A saved list with a removed control loads with that key dropped and a
visible notice.
- Another user cannot read or apply someone else's saved list.
- Saved lists appear in tenant export and are removed with the user.
