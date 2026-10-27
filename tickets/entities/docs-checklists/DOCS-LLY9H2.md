---
id: DOCS-LLY9H2
type: docs-checklist
title: 'Docs: dashboard cards (TKT-2SGPJZ)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (gate-before-fold order in dashboard_card.go, unplaced rule and severity order in dashboard_fold.go, per-card error handling in DashboardView.vue, list cards read-only in Configure)
- [x] Function/type docs if public API (DashboardCardValue wire type, ToneColor, validateRatioCard, validateTableCard, foldMatrix, foldRatio)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing gate-before-fold and collection-reads rules)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-data-entry Dashboard section: list source, matrix with tone_by and colors, ratio with match and group_by, validation rules, per-card failure, Configure limitation; regenerated docs/data-entry.md)
