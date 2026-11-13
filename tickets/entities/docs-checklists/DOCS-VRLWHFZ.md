---
id: DOCS-VRLWHFZ
type: docs-checklist
title: 'Docs: Structured questions in realtime channels'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: `PlanBatch` (decide until the first move, the asked set), `limiter.takeAll` (all or nothing), `groupQuestions` (a notice inside a batch run), the removal rule in `TestPostBatch_RemovingLastOpenQuestionDoesNotMove`
- [x] Function/type docs if public API: godoc on `Asked`, `AppendBatch`, `PostBatch`, `MaxBatch`, `ErrTooManyAsked`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela channel post-batch` help and `GUIDE-cli-reference.md`

## User-facing Documentation

- [x] `GUIDE-realtime.md`: "Asking several questions", `session.open_questions`, the `asked_question` and `too_many_questions` refusals, the `_batch` route, removal of an open question; `GUIDE-data-entry.md`: "Options as cards"; `GUIDE-mcp-server.md`: `post_messages`; regenerated with `just docs`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI `postBatch` operation, `asked` and `batch` schemas, the 409 description
