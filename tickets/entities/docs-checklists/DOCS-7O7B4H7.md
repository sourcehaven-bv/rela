---
id: DOCS-7O7B4H7
type: docs-checklist
title: 'Docs: TKT-UP0AYVN'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the selectable rule in EntityList and why it now reads canAddToPile
- [x] Function/type docs if public API: godoc on `ListButtonsConfig`, `Config.ListButtons`, `SidebarListButtons` and `listButtonsWire`

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `docs/data-entry.md`: new section "List and search buttons (`list_buttons`)", the overview example, the header_buttons exclusion bullet and the piles guide; mirror regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: a UX filter; no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: /_sidebar gains a field documented in the Go wire type; the guide names it)
