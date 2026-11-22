---
id: TKT-FJO08Z4
type: ticket
title: Name the page and the app in the browser tab title
kind: enhancement
priority: medium
effort: s
started: "2026-11-19"
completed: "2026-11-19"
status: done
description: Every tab of the web app is titled rela whatever the page and whatever app.name says. The tab title should read <page> · <app name> so customers can tell tabs apart and see whose portal it is.
---

## Description

The browser tab title is always `rela`: `index.html` sets it and nothing changes
it except the inbox count. Customers of a portal see `rela` in every tab, not
the portal name, and cannot tell tabs apart.

## Acceptance criteria

- The tab title reads `<page> · <app name>`, with `app.name` from
data-entry.yaml.
- The page part is the title the page shows: the list, view, dashboard,
form or entity title, or the name of a built-in screen.
- A screen without a title shows the app name alone.
- The inbox count stays in front: `(N) <page> · <app name>`.
- Documented under `app.name`.
