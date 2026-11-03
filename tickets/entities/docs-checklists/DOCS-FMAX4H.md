---
id: DOCS-FMAX4H
type: docs-checklist
title: 'Docs: Sessions 4/1: session config and message-driven transitions'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the decider under the store lock (`internal/realtime/session.go`), the file-tier crash windows in the filechannels package doc, the merge-key refusal in `realtimecfg/keys.go`, and `predicate.ErrNilOperand`
- [x] Function/type docs if public API: godoc on `channels.Session`, the session step and outcome types, `realtimecfg.Session` and its accessors, `MessageView.IsSessionNotice`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: only the generated feature list changed through `just docs`)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela channel` header and `post` output show the session

## User-facing Documentation

- [x] `GUIDE-realtime.md`: new Sessions section (states, `final: true`, `on:` transitions, `roles`, `by: trigger`, `when:` guards with `~=`, notices, locked causing messages), load checks, HTTP session block, 409 `session_notice` and `session_message`, agents, config reload and stranded sessions; regenerated into `docs/realtime.md`
- [x] `GUIDE-mcp-server.md`: `read_channel` and `post_message` session fields; regenerated into `docs/mcp-server.md`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI `ChannelSession` schema and the `session` property on Channel; MCP tool descriptions
