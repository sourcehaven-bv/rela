---
id: DOCS-SNZWYI3
type: docs-checklist
title: 'Docs: TKT-0101IO4'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why edit mode is type=text, why the link stops click propagation, why mailto drops its query, why the kanban card skips a nested anchor
- [x] Function/type docs if public API: godoc on `PropertyDisplay.Link` and `WidgetURL`; TSDoc on `linkHref`, `opensInNewTab` and `viewFieldRoutingHintFor`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: uses the existing widget registry pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-metamodel.md` (property table row, "Display formats" split into Currency and Link with the scheme rule and surfaces) and `GUIDE-data-entry.md` (`url` widget row and when to use it); regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change; the scheme allowlist is described in the metamodel guide)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: the OpenAPI spec does not describe the `display` object)
