---
id: DOCS-HSS1
type: docs-checklist
title: 'Docs: Sync data-entry list filters with URL query params'
status: done
---

## Documentation Updates

- [x] `docs/data-entry.md` — new "URL Sync for Filters" section covering:
  - Bracket-format URL examples (equality, operator, multi-value array form)
  - Full operator list (eq, ne, contains, in, lt, lte, gt, gte) with reference to backend source
  - Fail-closed behavior for unknown operators
  - Static filter collision granularity (whole-property lock) with workaround
  - Text-input debounce behavior (250ms)
  - Clear-filters preservation of non-filter params
  - Multi-value `in`/`ne` vs last-write-wins asymmetry
- [x] ~~CLI help text~~ (N/A — frontend-only feature)
- [x] ~~CLAUDE.md~~ (N/A — no workflow changes)
- [x] ~~README.md~~ (N/A — feature-level docs live in docs/data-entry.md)
