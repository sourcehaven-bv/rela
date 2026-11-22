---
id: TKT-3VLKP3D
type: ticket
title: 'Portal UX fixes: date picker and mobile menu bar and select-all label and phone list columns'
kind: enhancement
priority: medium
effort: m
started: "2026-11-20"
completed: "2026-11-20"
status: done
---

## Description

Findings from an end-to-end test of a customer portal (harbourmaster).

1. The as-of date picker in the sidebar header could not be hidden, sat in
the switchers' row and squeezed the space name to a few letters, and showed the
browser's own placeholder (`dd-mm-jjjj`) in the browser's language. It now has a
row of its own, shows "Now" until a date is picked, and `builtins: {as_of: ...}`
hides or permission-gates it per space, like the other built-in rows.
2. On a phone the floating hamburger overlapped the next-action banner
and sticky table headers; content scrolled past around it. It now sits in an
opaque full-width sticky bar, and on topbar pages with a banner the bar keeps
its row so the banner starts below it.
3. The select-all checkbox of a list without sections was labelled
"Select all in " (cut off). An empty section name now gives "Select all" ("Alles
selecteren").
4. On a phone a list shows only its title and enum columns, so an
invoice list lost its total and PDF link. A list column's `primary:` now
overrides the default either way. Lists only; refused on dashboard card and view
section columns.

## Acceptance criteria

- `builtins: {as_of: {hidden: true}}` removes the picker; `permission:`
gates it; other rows unaffected.
- The space name is not truncated by the picker; "Now" opens the picker.
- On a 375px viewport the hamburger never overlaps a banner or content.
- The select-all label of an unsectioned list is "Select all".
- `primary: true` keeps a column in the compressed list; `primary: false`
drops an enum column; `primary` on a card or view column fails to load.
