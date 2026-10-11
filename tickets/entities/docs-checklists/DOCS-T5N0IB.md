---
id: DOCS-T5N0IB
type: docs-checklist
title: 'Docs: Gantt horizontal scroll with Now and arrow navigation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols documented: `PX_PER_DAY`, `SCROLL_UNIT_DAYS`, `withToday`
and `pct` in `ganttLayout.ts` carry doc comments
- [x] Non-obvious decisions carry a WHY: the timeline cap (`pxPerDay`), the
inclusive end (`pos(end + 1)`), `overflow: clip` versus hidden, the sticky label
track, scroll anchoring and the dropped stale fetch
- [x] ~~`CLAUDE.md` updated~~ (N/A: no new project-wide rule)

## Project Documentation

- [x] `GUIDE-data-entry.md` (generates `docs/data-entry.md`): new "Scrolling
through time" section covering day widths, Now, the arrows, zoom anchoring,
inclusive end dates and the compressed flag
- [x] ~~`docs/data-entry/api-reference.md`~~ (N/A: no API change; the `face`
field moves to TKT-7W9LKE)
- [x] ~~`README.md`~~ (N/A: no top-level feature list change)

## External Documentation

- [x] ~~Tool README~~ (N/A: no external tool touched)
- [x] ~~Migration notes~~ (N/A: no config change)

## Verification

- [x] The documented behaviour runs in `e2e/tests/gantt-scroll.spec.ts`
- [x] `just docs` regenerated `docs/data-entry.md`
