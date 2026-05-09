---
id: DOCS-TM2LG
type: docs-checklist
title: 'Documentation: Quick-search/jump command palette'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] New component has a top-of-file comment explaining purpose, behavior, and notable design decisions (focus restoration, Escape stopPropagation, Tab trap rationale).
- [x] `searchEntities` JSDoc explains the new optional `signal: AbortSignal` parameter and its use case.
- [x] Tab handler comment documents the limitation: "When more controls are added (clear button, filter chips), swap this for a proper focus trap."
- [x] Named constants (`DEBOUNCE_MS`, `MIN_QUERY_LEN`, `MAX_RESULTS`) carry inline justifications.

## Project Documentation

- [x] In-app keyboard-shortcuts modal (`KeyboardShortcutsModal.vue`) — new row "Cmd/Ctrl+K — Quick jump" under Global section. This is the user-facing reference for the SPA's keyboard surface.
- [x] ~~User guide / reference docs~~ (N/A: no separate user guide for the SPA)
- [x] ~~CLI help text~~ (N/A: no CLI surface)
- [x] ~~CLAUDE.md~~ (N/A: no new patterns introduced; uses existing modalStack + ConfirmModal patterns)
- [x] ~~README.md~~ (N/A: no project-level changes)

## External Documentation

- [x] ~~Public API documentation~~ (N/A: no public API change. The `searchEntities` signature change is frontend-internal — `/api/v1/_search` is unchanged.)
- [x] ~~Migration notes~~ (N/A: additive feature; no breaking changes)
- [x] ~~Changelog~~ (N/A: project does not maintain a changelog file)
