---
id: DOCS-E5LHOYM
type: docs-checklist
title: 'Docs: TKT-31822PN'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why a gate is a UX filter and not access control, why an entity dashboard never shows the validation card, why toggle errors carry the line number
- [x] Function/type docs if public API: godoc on `ChromeToggle`, `EntityHeaderConfig`, `SidebarEntityHeader` and `DashboardResponse.Validation`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing card `permission:` pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change; `rela acl audit` counts the new permissions without new flags)

## User-facing Documentation

- [x] `GUIDE-data-entry.md`: `validation` row in the dashboard fields table, sections "Hiding the validation card (`validation`)" and "Entity header buttons (`entity_header`)", `entity_header` in the file structure; both say the permission gate is a UX filter; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change; the history and pile endpoints keep their own ACL)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: docs/data-entry/api-reference.md does not document `/_sidebar` or `/_dashboard`)
