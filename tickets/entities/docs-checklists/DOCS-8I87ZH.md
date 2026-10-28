---
id: DOCS-8I87ZH
type: docs-checklist
title: 'Docs: Session composer: offer only message types that apply in the current state'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: `SessionStep.Admit` contract, `ErrSessionState`, the pre-check versus store check order in `Service.Post`, trigger skip on refusal
- [x] Function/type docs if public API: godoc on `MessageType.In`, `PostableIn`, `SessionStateError`, `ReadView.MessageTypes[].CanPost`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela channel post` shows the refusal text; `rela validate` prints the `in:` load errors

## User-facing Documentation

- [x] `GUIDE-realtime.md`: "Where a message type may be posted" with a tested example, load checks, 409 `session_wrong_state`, stranded sessions, edits, `when_closed: post`, every process needs this release; `GUIDE-mcp-server.md`: `can_post` and the refusal; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change; a session-state rule, not access control)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI 409 `session_wrong_state` on the post route; MCP tool text
