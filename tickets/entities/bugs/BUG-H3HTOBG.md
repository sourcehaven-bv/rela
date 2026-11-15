---
id: BUG-H3HTOBG
type: bug
title: Slash shortcut opens /search on a list page
description: The sidebar's / guard used a stale selector and missed the list's search box.
priority: low
status: backlog
---

## Summary

The Sidebar's `/` shortcut should leave a list page alone, because the list has
its own search box. Its guard looked for `.entity-list .search-box`. The shell
(`PageHeaderContent`) draws the list header outside `.entity-list`, so the
selector never matched and `/` opened `/search`.

## Fix

The guard matches `.search-box input, .search-form input` and counts only a box
with client rects. `Sidebar.shortcut.test.ts` builds the header outside the
list, as the shell does, and stubs the hidden case.
