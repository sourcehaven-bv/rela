---
id: FEAT-9F2S23
type: feature
title: 'Pages: one sidebar destination with tabbed views (Board, Table, Timeline)'
description: Group existing screens as tabs of one page with its own URL (/p/<page>/<tab>), so Board, Table and Timeline of the same work are one sidebar destination and a bookmark always shows the same tab bar.
status: proposed
---

## Summary

Group existing screens (list, kanban, gantt, calendar, dashboard, document) as
tabs of one page, so a user switches between Board, Table and Timeline of the
same work without going back to the sidebar. The page has its own URL, which
names both the page and the tab, so a bookmark always shows the same tab bar.

The tab bar is the library's `RlViewTabs` in `RlPageHeader`'s `#tabs` slot.

## Later

- Search text and simple filters carried across tabs that show the same
entity type.
- Views scoped to one parent entity (an initiative's tasks), filled from the
URL.
- User-created tabs (the library's "+ Add").
