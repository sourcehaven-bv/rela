---
id: DOCS-8ETNQEC
type: docs-checklist
title: 'Docs: TKT-3U99O4L'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: template.go explains the button shape (link in a colored cell); ManualStarts explains why a failing condition offers no start
- [x] Function/type docs if public API: godoc on `flowengine.ManualStarts`, `mailrender.Button`, `mailrender.ErrButton`, `Message.Button` and `DashboardCard.HideEmpty`

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] GUIDE-flows documents `when:` on a manual start; GUIDE-mail and GUIDE-lua-scripting document `button`; GUIDE-data-entry documents `hide_empty` on cards; `docs/` regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no change to authorization; the button URL uses the existing link allowlist)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no new endpoint; the dashboard config gains a documented key)
