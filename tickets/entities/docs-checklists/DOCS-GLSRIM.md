---
id: DOCS-GLSRIM
type: docs-checklist
title: 'Docs: Realtime 3/7: SPA channels'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the cursor and paging rules in `useChannel`, the sanitizer allowlist in `renderMessageMarkdown`, the edit snapshot in the panel, the per-message affordance checks in `internal/realtime/msgops.go`, the reload coherence of realtime.yaml and data-entry.yaml
- [x] Function/type docs if public API: godoc on `Service.Describe` and the channel schema types; JSDoc on the library component props, including the `entityHref` contract and the sanitized `html` prop; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change; the rela-components README has a new Channels section)
- [x] CLAUDE.md updated (if new patterns): `frontend/CLAUDE.md` says message markdown goes through `renderMessageMarkdown`, never `renderMarkdown`
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no new CLI commands; `rela validate` gained the `channels:` check, which reports as a ✗ line)

## User-facing Documentation

- [x] `GUIDE-realtime.md`: lifecycle labels, reserved operation names with an upgrade note, per-message actions, the channel schema endpoint, character counting for `max`, and an "In the web app" section; regenerated into `docs/realtime.md`
- [x] `GUIDE-data-entry.md`: the `channels:` presentation section; regenerated into `docs/`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): the OpenAPI document describes `GET /api/v1/_schema/channels`, the `ChannelSchema` component and the new per-message `_actions`, `edit_until` and `release_after` fields
