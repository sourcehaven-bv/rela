---
id: DOCS-XE5WEA
type: docs-checklist
title: 'Docs: Guarded branching transitions per message type'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: first-match decider, fail-closed guard errors, the `{active, from, to}` outputs gate, trigger-shadow flag, mark lookup by job flag
- [x] Function/type docs if public API: godoc on `Session.Branches`, `TransitionTo`, `Transition.Admits`, `MaxTransitionBranches`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## User-facing Documentation

- [x] `GUIDE-realtime.md`: "Several targets for one message type", `on` table, limits, load checks, guard errors, web app hint and form decisions, agent notes; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI `ChannelSession.transitions` description; MCP `read_channel`, `read_session`, `list_inbox`, `post_message` text
