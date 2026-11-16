---
id: DOCS-KGKYTPI
type: docs-checklist
title: 'Docs: Store-side paging for list_entities filters that do not lower'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: QueryConditions and its Lower invariant, ConditionLowerer, PageQuery.Related trust contract, listPage.filterInGo residual shapes
- [x] Function/type docs if public API: appbuild.QueryConditions, GatedReadBundle.Conditions, mcp.ConditionLowerer, RelatedReads.Conditions, visibility.PageQuery.Related, ErrPropsNotPushable

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing consumer-side interface and gated-read patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no rela CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (if applicable): GUIDE-mcp-server (and the generated docs/mcp-server.md) describes current_user filters and which filters the store answers
