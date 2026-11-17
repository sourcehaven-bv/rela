---
id: DOCS-A8JR2UF
type: docs-checklist
title: 'Docs: Store query contract: or, not and ordered comparisons'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (integerExpr decode steps, Cond.Constant, narrowed limits, NilOrderedIsFalse scope)
- [x] Function/type docs if public API (store.Cond, OrderKind, AndWhere, ValidateCond, Constant)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: the collection-reads rule already says a condition joins pushdown only when LowerCondition lowers it exactly)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)
- [x] GUIDE-data-entry and GUIDE-mcp-server updated in docs-project; docs/ regenerated

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository has no changelog; release notes come from PR titles)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API shape change)
