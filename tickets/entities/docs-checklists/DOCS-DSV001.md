---
id: DOCS-DSV001
type: docs-checklist
title: 'Docs: Create menu puts the current page''s type first (TKT-DSVLOQ)'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (`useViewEntityTypes.ts` says which views show which types and why several types lead in config order; `routeTabId` says why the two-segment entity page route has no tab; the SpaceCreateMenu header describes the order and the separator)
- [x] ~~Function/type docs if public API~~ (N/A: no public API change)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-data-entry.md § Spaces, after `create:`: the order rule, a table per page kind, several types in `create:` order, entity pages narrow then order, entity view and documents keep config order, no setting. Regenerated into docs/data-entry.md with scripts/generate-docs.sh)
