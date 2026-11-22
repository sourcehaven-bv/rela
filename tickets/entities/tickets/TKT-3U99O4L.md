---
id: TKT-3U99O4L
type: ticket
title: Manual start condition, mail button and hiding empty dashboard cards
kind: enhancement
priority: medium
effort: s
started: "2026-11-21"
completed: "2026-11-21"
status: done
description: 'Add when: to manual flow starts, a button to mail.render and hide_empty to dashboard cards.'
---

## Description

Three config gaps found while configuring a customer portal:

- A flow's `start: on: manual` takes no condition, so staff see "Start
Signup" on an account whose signup is already done.
- `mail.render` has no call to action, so a script that wants a "Complete
your signup" button has to fall back to hand-written HTML.
- A dashboard card cannot hide when its list is empty, so a customer sees
an empty "Open invoices" card.

## Acceptance criteria

- An `on: manual` start takes an optional `when:`, compiled with the
flow guard rules. Where it does not hold, the flow is not offered as startable
and a start is refused as a conflict. A caller without the permission is still
refused as forbidden.
- `mail.render{button = {label, url}}` renders a link styled as a button
below the intro, with a `label: URL` line in the text part. A missing label or
an unsafe URL raises.
- A dashboard card takes `hide_empty: true` and is left out when its list
or query has no rows for the reader. A failed card still shows. `hide_empty` on
a line card is refused at load.
- Documented in the flows, mail, Lua and data-entry guides.

## CI follow-up

- The God-object lint (plimsoll) failed on the extra `HideEmpty` field.
The `max-fields` directive on `DashboardCard` went from 22 to 23, under the
format-mirror exception documented on `Config`.
- CI also flagged a shadowed `err` in `FlowControl.start` (govet; the fix
uses `authErr`, which also satisfies gocritic sloppyReassign) and an
unclassified `hide_empty` key in the configedit allowlist. Both are fixed;
`hide_empty` is locked beside `width`.
