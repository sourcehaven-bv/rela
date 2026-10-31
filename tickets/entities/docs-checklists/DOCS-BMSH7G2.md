---
id: DOCS-BMSH7G2
type: docs-checklist
title: 'Docs: Card wall board with optional drag-to-group'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (`useWallGrouping` write order, one-at-a-time drops and refusals; `useBoardFlip` stale-width rule; `zoneAt` split; handle tab order)
- [x] Function/type docs if public API (`Kanban.Layout`, `LayoutWall`, `CardStyleNote` godoc; `useDraggableSection`, `BoardSectionDropPosition`, `Grouping` story in rela-components)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the README does not list kanban options)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog file; release notes come from PR titles)
- [x] API docs updated (if applicable) (`docs/data-entry.md`: `layout` field, `card.style`, new "Walls (`layout: wall`)" section covering plain walls, grouping, renaming, group moves, permissions and limits)
