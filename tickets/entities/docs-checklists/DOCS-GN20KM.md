---
id: DOCS-GN20KM
type: docs-checklist
title: 'Docs: Sessions 4/6 input_required/handoff'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: deadline-reset chain search and can-answer rule (`internal/realtimecfg/waiting.go`), holder resolution from the redacted anchor and per-reader `waiting_on_me` (`internal/realtime/waiting.go`)
- [x] Function/type docs if public API: godoc on `realtimecfg.SessionState` status/waiting_for, `realtime.SessionView` fields, `acl.AnyRoleGrants`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela channel` shows status and the waiting line with role labels

## User-facing Documentation

- [x] `GUIDE-realtime.md`: status, `waiting_for`, `waiting_on_me`, handoff and claim, deadline-reset rule with `since: session` as the remedy, stranded sessions; regenerated into `docs/realtime.md`
- [x] `GUIDE-mcp-server.md`: `read_session` and its use with `watch`; regenerated

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI session `status`, `waiting_for`, `waiting_on_me`, `stranded`, `no_session`
