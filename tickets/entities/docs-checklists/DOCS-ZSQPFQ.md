---
id: DOCS-ZSQPFQ
type: docs-checklist
title: 'Docs: TKT-S5OJ7S'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why every entry form keys its state, why the two authored-only checks cannot fire for a derived `in:`, the zero value of the `derived` flag
- [x] Function/type docs if public API: godoc on `InKey` and the `In` field; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-realtime.md` (`default_in` paragraph and example, authored `in:` as a union, stale full lists, stranded and trigger bullets) and `GUIDE-mcp-server.md` (`read_channel` and `post_message` rows); example pinned by `TestDocsExample_DefaultIn`; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: narrows where posts are allowed; no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI post description names `session.default_in`
