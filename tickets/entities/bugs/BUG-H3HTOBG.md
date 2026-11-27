---
id: BUG-H3HTOBG
type: bug
title: Slash shortcut opens /search on a list page
description: The sidebar's / guard used a stale selector and missed the list's search box.
priority: low
why1: Pressing / on a list page opened /search instead of focusing the list search box.
why2: The sidebar guard looked for .entity-list .search-box, but the shell draws the list header outside .entity-list.
why3: The unit test built the header inside the list, so it did not match the real layout.
why4: The shell layout changed after the guard was written, and the guard relied on a DOM path rather than on the element itself.
why5: 'Systemic: component tests built their own approximation of the page layout, so a layout change could not break them.'
prevention: The guard matches the visible search input, and the test builds the header outside the list as the shell does (#2357).
started: "2026-11-27"
completed: "2026-11-27"
status: done
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
