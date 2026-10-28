---
id: DOCS-DK9JEZ
type: docs-checklist
title: TKT-6RXAC0 docs
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: mention classification, the uniform post answer, eviction order, edit-adds-only rows, gate before the cut
- [x] Function/type docs if public API: godoc on `channels.Store` AppendAddressed and ListAddressed, `AddressedRow`, `realtime.MaxMentions`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): mentions in `rela channel inbox`

## User-facing Documentation

- [x] `GUIDE-realtime.md`: mentions, replies, caps, the 2000-row scan; `GUIDE-data-entry.md`: mention chips; `GUIDE-mcp-server.md` and `GUIDE-cli-reference.md`: inbox mention kinds; regenerated
- [x] `GUIDE-acl-security.md`: mention gating at read time and the break-glass reply case; regenerated

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI inbox `mentions`, `mention_count`, kinds and `too_many_mentions`
