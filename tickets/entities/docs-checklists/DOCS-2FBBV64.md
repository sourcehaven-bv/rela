---
id: DOCS-2FBBV64
type: docs-checklist
title: 'Docs: /_search cancels and search cost'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: textHitsCTE / textHitsJoin (why ids not faces, why an array and not a semi join), freeTextLimit guards, requestCanceled (request context only), SEARCH_DEBOUNCE_MS
- [x] ~~Function/type docs if public API~~ (N/A: no exported API added)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated: docs/data-entry/api-reference.md (when `limit` bounds the search), docs/server-security.md (`canceled=true`, status 499, `written_status`)
