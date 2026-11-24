---
id: DOCS-332MPBQ
type: docs-checklist
title: 'Docs: TKT-T0DK37'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: how `searchPlan.page` skips whole types by count before it sets an offset, why the free-text read asks for one hit past the cap, and why `total` is a lower bound for free text
- [x] Function/type docs if public API: godoc on `maxSearchResults`, `maxSearchPage`, `parseSearchPage`, `countQueries` and `ListMeta.Capped`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-acl-security.md` (the result-limit bullet now describes one page, `page`, `meta.total`, `meta.has_more` and `meta.capped`) and `GUIDE-data-entry.md` (bridge row `rela.search({query, type, page?})`); regenerated `docs/`
- [x] ~~Security guide~~ (N/A: totals and flags stay post-gate; no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: the repo has no OpenAPI spec for `/_search`; the guides above carry the contract)
