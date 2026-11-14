---
id: TKT-EIFFTLB
type: ticket
title: 'Mac conventions in rela-desktop: reopen after closing the last window, Find, Print, Show in Finder, Help'
kind: enhancement
priority: medium
effort: s
started: "2026-11-13"
status: planning
---

## Description

Make rela-desktop behave like a Mac app in the places where it did not.

A Mac app keeps running when its last window closes. rela-desktop held the main
window in a field, so after ⌘W the Dock icon, Open Recent, a Finder double
click, a second launch and Clone from Git all acted on a destroyed window and
showed nothing. The main window is now found by name and opened again when it is
gone. The saved maximized state is restored at launch.

The menu bar gains the items Mac users expect:

- Edit > Find… (⌘F) focuses the page's search box, or opens search.
- File > Print… (⌘P) prints the page; the app shell and page header have
print styles so the whole page prints without navigation.
- File > Show in Finder reveals the focused project's folder or .rela file.
- Help > Rela Help opens the documentation.

## Acceptance criteria

- [ ] Closing every window and clicking the Dock icon opens a window.
- [ ] Open Recent and Finder opens work with no window open.
- [x] Find, Print, Show in Finder and Rela Help are in the menu bar.
- [ ] Printing a list shows the full list without the sidebar.
