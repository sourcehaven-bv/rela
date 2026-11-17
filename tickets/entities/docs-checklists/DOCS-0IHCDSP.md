---
id: DOCS-0IHCDSP
type: docs-checklist
title: 'Docs: TKT-MWZF0UO'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the done state stays off the sidebar, why an empty step is not done, why `_page_status` is world-capable, per-step cost
- [x] Function/type docs if public API: godoc on `PageTab.DoneWhen`, `PageDoneEntries`, `ViewConditionPageDone`, `handleV1PageStatus`, `PageStatusResponse`

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing nav-status pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md` "Marking a step done": syntax with `~=`, empty-list rule, advisory, per-user and hidden-property behaviour, world, `/_page_status` shape; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: judged over the gated rows; no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): `/_page_status` described in the guide; no OpenAPI entry exists for the sibling `/_nav_status` either
