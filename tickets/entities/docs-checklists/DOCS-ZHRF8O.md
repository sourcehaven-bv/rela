---
id: DOCS-ZHRF8O
type: docs-checklist
title: 'Docs: Gantt drag bars to move and resize'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols documented: `useGanttDrag`, `canDrag`, `shift`,
`nodeAddress`, `GanttDragDeps`, `GanttPreview` and the wire field
`GanttNode.Face` carry doc comments
- [x] Non-obvious decisions carry a WHY: the lazy verdict (why the gantt
response has no write affordance), the re-read at commit, why the preview
records its origin, the preview held until the reload, click suppression
- [x] ~~`CLAUDE.md` updated~~ (N/A: no new project-wide rule)

## Project Documentation

- [x] `GUIDE-data-entry.md` (generates `docs/data-entry.md`): new
"Rescheduling by dragging" section covering move, edges, keys, who sees the
handles, preconditions and `face` on the wire
- [x] ~~`docs/data-entry/api-reference.md`~~ (N/A: that file does not cover
the gantt endpoint; its wire shape is documented in the guide)
- [x] ~~`README.md`~~ (N/A: no top-level feature list change)

## External Documentation

- [x] ~~Tool README~~ (N/A: no external tool touched)
- [x] ~~Migration notes~~ (N/A: no config change)

## Verification

- [x] The documented behaviour runs in `e2e/tests/gantt-drag.spec.ts`
- [x] `just docs` regenerated `docs/data-entry.md`
