---
id: DOCS-AJ6F6F
type: docs-checklist
title: 'Docs: Entity references to faced types never resolve as mentions'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: `collectMentions` doc and body comment (world resolution, gate, redaction); `rewriteEntityRefToken` and `RenderMarkdownOptions.world`; `viewWorld` in EntityDetail.vue
- [x] Function/type docs if public API: `ViewResponse.Mentions` (apiwire v1) documents that mentions resolve in the request's world

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] CLAUDE.md updated: `frontend/CLAUDE.md` now names `ResolveIDsErr` in the request's world as the source of mention titles
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PR titles)
- [x] API docs updated: `docs/data-entry.md` and `docs-project/entities/guides/GUIDE-data-entry.md` explain that a reference resolves in the reader's world and links into it
