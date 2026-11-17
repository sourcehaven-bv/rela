---
id: DOCS-DGKT7TR
type: docs-checklist
title: 'Docs: TKT-WJP6PBM'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: `RelationDefaultPicker` doc on the read order, 404 fallthrough and the not-found and failed marks; `EntityPickerModal` `types` and `maxResults` docs; `APIRelationDef` doc on why it carries types, not targets
- [x] Function/type docs if public API: godoc on `APIRelationDef.TargetType` and `TargetTypes`; comment-lint gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md` Relation Defaults paragraph says the target is chosen by search; `docs/data-entry.md` carries the same text

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: `/_settings` has no OpenAPI entry; the wire shape is pinned by the golden corpus)
