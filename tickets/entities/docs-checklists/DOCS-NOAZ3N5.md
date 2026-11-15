---
id: DOCS-NOAZ3N5
type: docs-checklist
title: 'Docs: TKT-D3RAB0E'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why `planSearch` declines each shape, why the page counts only when full, why the position window uses `orderKeySQL`
- [x] Function/type docs if public API: godoc on `searchPlan`, `planSearch`, `scopedQuery`, `searchPage`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing collection-reads rule)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] ~~User guides updated~~ (N/A: no guide documents `/_search` meta.total; the change is recorded in the ticket and PR body)
- [x] ~~Security guide~~ (N/A: no ACL change; gating stays inside the store query)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: `ListMeta.Total` carries no per-endpoint description)
