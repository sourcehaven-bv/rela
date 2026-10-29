---
id: DOCS-4F45WY
type: docs-checklist
title: 'Docs: Session decisions as one-click buttons'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: generation captured at the click, per-click lock and channel-switch release, `actionsGeneration`, check order in `Service.Post`
- [x] Function/type docs if public API: godoc on `PostInput.ExpectGeneration`, `SessionGenerationError`, `ChannelDecisionView`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela channel post --expect-generation`; `GUIDE-cli-reference.md`

## User-facing Documentation

- [x] `GUIDE-realtime.md`: decision buttons, `expect_generation` and 409 `session_moved`, post check order, advice to give decision types a `label:` and an `in:`, agents send the generation; `GUIDE-mcp-server.md`: `post_message` `expect_generation`; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI `expect_generation` and 409 `session_moved` on the post route
