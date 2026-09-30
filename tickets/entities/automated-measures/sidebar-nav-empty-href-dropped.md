---
id: sidebar-nav-empty-href-dropped
type: automated-measure
title: 'Test: a nav entry with an empty href is dropped, never rendered as a link'
description: Asserts toNavGroups drops a sidebar entry with neither an action nor a non-empty href, and keeps an action entry whose empty href is legitimate.
kind: test
location: frontend/src/components/common/sidebarNav.test.ts
status: active
---

## Description

`toNavGroups` drops a sidebar entry that has neither an action nor a non-empty
href, so it can never be rendered as a `RouterLink` with `to: ''`.

That target resolves against the CURRENT location, so such a row matches every
route and steals the active highlight from the row that should have it. The
defect is silent: the row typechecks, renders, and looks ordinary.

Two tests hold the pair of cases apart, which is the part worth keeping:

- `drops an entry with an empty href, which would match every route`
- `keeps an action with no href, which is navigable by other means`

The second matters because an empty `Href` is CORRECT for an action entry — the
server sets it deliberately so the frontend renders a button. A guard that
looked only at `href` would delete every action from the sidebar, trading a
wrong highlight for missing functionality.
