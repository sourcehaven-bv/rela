---
id: DOCS-LN0HZH
type: docs-checklist
title: 'Docs: Agent activity indicator on channels and boards'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: batch shape and gating order in the busy reader, the name filter on the waiting query, why a busy field may be "not served", the shared minute clock
- [x] Function/type docs if public API: godoc on `Session.WorkingStates`, `SessionState.StatusDeclared`, the busy reader and `BusyAnchor`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing collection-reads rule)
- [x] Help text accurate (if CLI changes): `rela validate` reports busy-field problems; MCP `read_session` and `post_message` descriptions

## User-facing Documentation

- [x] `GUIDE-realtime.md` "Busy indicator" (working states, stall timer example, adding a session to a type) and `GUIDE-data-entry.md` "Busy cards"; example pinned by `TestDocsExample_Busy`; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change; `_busy` follows the row gate and the channel `read:` role)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): `docs/data-entry/api-reference.md` `?busy=` and `_busy`
