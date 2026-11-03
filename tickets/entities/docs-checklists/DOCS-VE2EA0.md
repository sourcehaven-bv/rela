---
id: DOCS-VE2EA0
type: docs-checklist
title: 'Docs: TKT-CPZKPB'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why `in` is copied, what `in_derived` means, and that only `_actions.post` decides whether a person may post
- [x] Function/type docs if public API: godoc on `MessageType.InDerived` and the new view fields; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-realtime.md` documents `in` and `in_derived` under `_schema/channels` and in "Take `in:` from the transitions"; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] OpenAPI message type schema documents both fields; MCP `read_channel` and `post_message` descriptions mention them
