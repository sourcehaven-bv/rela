---
id: DOCS-VEB8AX4
type: docs-checklist
title: 'Docs: TKT-CBRLZWU'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the UX gates count a role relation and the security gates do not (on `access.Mode.PermitsGatedUIElement`), why the memo keeps no ctx error, the zero tail in `HasOutgoingEdge`
- [x] Function/type docs if public API: godoc on `acl.HoldsPermissionOnAnyEntity`, `access.UIPermissionHolder` and `aclstore.Graph.HasOutgoingEdge`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md` (paragraph and acl.yaml / data-entry.yaml example under hiding entries) and `GUIDE-acl-security.md` (which gates count a role relation and which stay global-only); regenerated with `just docs`
- [x] Security guide: `GUIDE-acl-security.md` updated as above

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API shape change)
