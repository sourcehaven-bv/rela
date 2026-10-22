---
id: DOCS-BTYQ6S
type: docs-checklist
title: 'Docs: Sessions 4/2: UI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the per-call role cache and the creator-role exception (`internal/realtime/session.go`, `authz.go`), the stepper precedence and Loading state (`channelView.ts`), and the view reuse in `OpenChannel.vue`
- [x] Function/type docs if public API: godoc on the session descriptor and `_actions.transitions`; component docs and props for `RlChannelSession`, the composer hint and `srHint`; comment-lint `doclink` gate clean

## Project Documentation

- [x] rela-components `README.md`: the stepper, composer ordering and the new caption keys
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-realtime.md`: the session in the browser (stepper, Paused, Ended, Stranded, Loading), composer ordering and the default selection, notices and announcements, the session block on `GET /api/v1/_schema/channels`, `_actions.transitions` and `guarded`; regenerated into `docs/realtime.md`
- [x] `GUIDE-mcp-server.md`: `_actions.transitions` on `read_channel`; regenerated into `docs/mcp-server.md`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI session descriptor, `guarded` and `_actions.transitions`
