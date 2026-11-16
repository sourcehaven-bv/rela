---
id: DOCS-48IRUBM
type: docs-checklist
title: 'Docs: TKT-M07C97Y'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the walk holds PageDown while loading, why the detail waits for the position, why `list_id` applies the list's condition to `/_position`
- [x] Function/type docs if public API: godoc on `PageTab.Walk`, `ScopeDescriptor.ListID`, `SidebarPageTab.Walk`; doc comments on `useStepWalk`, `walkScope` and `PageWalk`

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md` "One row at a time": config, URL, missing row, present-mode keys; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: reads go through the existing gated list and position endpoints)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated~~ (N/A: `/_position` scope fields are documented in the guide; no OpenAPI entry exists for it)
