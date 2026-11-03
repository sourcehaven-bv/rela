---
id: DOCS-V1UTGY
type: docs-checklist
title: 'Docs: Built-in typing indicator per channel type'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the typing store's heap and per-channel pruning (`internal/realtime/typingstore.go`), the wake rules (start, post, expiry only), and `useTyping`'s throttle and settle
- [x] Function/type docs if public API: godoc on `Service.Typing`, the typing store, `realtimelimits.TypingTTL`/`TypingRate`, `ReadView.TypingNow`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-realtime.md`: `typing: true`, who may send, `typing_now` in the head, `_typing` route and its refusals, rate and shared budget, the reserved `typing` message type name; regenerated into `docs/realtime.md`
- [x] `GUIDE-mcp-server.md`: `typing_now` on `read_channel`; regenerated into `docs/mcp-server.md`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI `_typing` path, `typing_now` on the head, `typing` on the schema and `_actions`
