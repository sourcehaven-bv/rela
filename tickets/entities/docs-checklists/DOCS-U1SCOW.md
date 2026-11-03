---
id: DOCS-U1SCOW
type: docs-checklist
title: 'Docs: matrix axis range and integer min/max (TKT-BRAM1A)'
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (axis fill vs. sparse fallback, why native bounds drop out of range, overflow-safe width)
- [x] Function/type docs if public API (PropertyDef.Min/Max/InRange, MatrixAxisTooWide, v1.PropertyDef and v1.DashboardMatrix fields, WidgetProps.max)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting rule)
- [x] Help text accurate (if CLI changes) (rela schema TYPE prints the range; no new flags)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-metamodel: integer ranges and the property options table; GUIDE-data-entry: matrix range, titles, order and the card options table; docs/ regenerated with just docs)
