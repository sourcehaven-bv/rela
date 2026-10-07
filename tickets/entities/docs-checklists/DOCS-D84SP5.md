---
id: DOCS-D84SP5
type: docs-checklist
title: 'Docs: background Lua automation actions'
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: `automationJobs` godoc covers delivery, saves during a run, the follow-up after a collapsed enqueue, and authority; `updatedTriggerErrors` says why a foreground script under `on.updated` is refused.
- [x] Function/type docs if public API: `WithBackgroundAutomationJobs`, `autocascade.BackgroundScripts`, `metamodel.JobRetry`, `principal.UserAutomation`, `principal.ToolAutomationJob`, `automation.EventEntityRenamed`.

## Project Documentation

- [x] ~~README updated~~ (N/A: no README-level feature)
- [x] ~~CLAUDE.md updated~~ (N/A: the jobs rules in CLAUDE.md already cover the queue seam; behavior is documented in docs/metamodel.md)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] Changelog entry added: docs/metamodel.md (from GUIDE-metamodel) documents the `updated` trigger and the "Run a Lua script in the background" section.
- [x] ~~API docs updated~~ (N/A: no API changes)
