---
id: TKT-VKM63H
type: ticket
title: 'navigation: entries are not validated for exactly one kind'
kind: enhancement
priority: low
effort: s
status: backlog
---

## Problem

`validateNavEntry` (`internal/dataentryconfig/validate.go`) checks that each
*referenced* list/kanban/action/document exists, but never checks that an entry
sets **exactly one** kind, nor that `label:` is non-empty.

So both of these pass validation today:

```yaml
navigation:
  - label: "Broken"            # no kind at all
  - label: "Ambiguous"         # two kinds
    list: tickets
    kanban: board
```

The first produces a sidebar item with an empty `href`; the second silently
resolves by the branch order in `navEntryToSidebarItem`
(`internal/dataentry/views_handler.go`), which is an implementation detail
rather than a documented precedence.

## Found during

TKT-M1AX6P, which added `document:` as a fifth named kind — widening the hole by
one. Deliberately left out of scope there to keep that change reviewable.

## Proposal

In `validateNavEntry`, count the set kinds (`list`, `kanban`, `dashboard`,
`search`, `settings`, `action`, `document`) on non-group entries and error on
zero or more than one. Require a non-empty `label:` for entries that render as a
link or button.

Check the in-tree configs first (`tickets/data-entry.yaml`,
`prototypes/data-entry/project/data-entry.yaml`, `docs-project/`) — if any
currently rely on the lax behaviour, this becomes a breaking config change and
should ship with a clear error message naming the offending entry.
