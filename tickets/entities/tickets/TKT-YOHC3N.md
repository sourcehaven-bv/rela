---
id: TKT-YOHC3N
type: ticket
title: 'History/diff views: put selected versions in the URL so a diff is shareable'
kind: enhancement
priority: medium
effort: s
status: done
---

## Problem

`HistoryView.vue` and `RelationHistoryView.vue` keep the compared versions in
local refs (`baseSel` / `targetSel`, typed `number | 'current'`). Nothing is
read from or written to the URL, so:

- A user cannot share a link to a specific diff ("look at v3 → v7").
- Reloading `/history/features/FEAT-1` silently resets to the default pair.
- Browser back/forward does not move between compared versions.

Every other stateful surface in the SPA already mirrors its state into the URL
(`useUrlFilterSync`, `useFormWizard`'s `?step=N`, `DocumentsPanel`'s `?doc=`),
so the history views are the outlier.

## Scope

Sync the two selected sides to the URL for **both** history views, following the
established seed / `router.replace` / echo-guard pattern.

## Origin

User request: "history / diff view should use query/path params for the versions
so a user can share a link to a specific diff".
