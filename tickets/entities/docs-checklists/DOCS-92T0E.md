---
id: DOCS-92T0E
type: docs-checklist
title: 'Documentation: Add search interface to data-entry list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — added context comments in `freeTextIDsForType`, `runFreeTextSearchE`, `AdHocFilterMenu.mode` prop, `useUrlFilterSync` q-readonly contract, and the Sidebar/`useKeyboardShortcuts` slash-key defer logic
- [x] Function/type docs if public API — `freeTextIDsForTypeResult` struct documents the (IDs, HasFilter) contract

## Project Documentation

- [x] ~~README updated~~ (N/A: in-app feature, no project-level surface)
- [x] ~~CLAUDE.md updated~~ (N/A: no new architectural pattern; existing rules apply)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: project does not maintain a changelog file; PR description serves this role)
- [x] ~~API docs updated~~ (N/A: `?q=` follows existing query-param conventions of `/api/v1/{plural}`; no separate API docs file in this project)
