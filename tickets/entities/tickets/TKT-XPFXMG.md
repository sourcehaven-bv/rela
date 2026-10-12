---
id: TKT-XPFXMG
type: ticket
title: Create menu on an entity page offers only types it can link
kind: enhancement
priority: medium
effort: s
status: ready
---

## Description

The space Create menu shows the same `create:` types on every page. On an entity
page it links a new item to the page's entity when the page offers a relation
for the type (BUG-PFLS22). A type the page cannot link is still offered, and an
item created from it is left unlinked. Example: on a topic page whose tabs
follow `bestaat_uit`, the menu offers `initiatief` and
`verantwoordelijkheidsgebied` next to `taak` and `project`.

On an entity page, the menu offers only the types for which a link resolves on
the open tab, by the existing rules in `pageCreateLink`. Outside an entity page
the menu is unchanged. When no type resolves, the Create button is hidden, as it
is for a space without `create:`.

The filter is in the frontend: the server does not know which page is open, and
the link data is already in the sidebar payload.

## Acceptance criteria

- On an entity page the menu lists only types a link resolves for on the open
tab; the list can differ per tab.
- Off an entity page the menu lists every `create:` type.
- With no linkable type, the Create button is hidden.
- An open create dialog survives a navigation that empties the menu.
- docs/data-entry.md § Entity pages describes the behaviour.
