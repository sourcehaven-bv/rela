---
id: TKT-XWHLJ3U
type: ticket
title: Remove dead code and duplicated helpers
kind: chore
priority: low
effort: s
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Description
Remove unused test helpers and options found by deadcode, replace about 19
sortedKeys copies with slices.Sorted(maps.Keys), and inline slices.Contains
wrappers.
