---
id: DOCS-6GJYSF
type: docs-checklist
title: 'Docs: dashboard card width (TKT-H21TYS)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic is not obvious (the container-query clamp and its 580px derivation, the no-dense decision in DashboardView.vue; why width is a named size on DashboardCard.Width)
- [x] Function/type docs if public API (DashboardCard.Width, validateDashboardCardWidth, the SPA DashboardCard.width type)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (GUIDE-data-entry: width row in Card Options, new "Card width" section with values, example, phone behaviour, no height setting, ordering and holes, Configure note; regenerated docs/data-entry.md)
