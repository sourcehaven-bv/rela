---
id: DOCS-1T12H4L
type: docs-checklist
title: 'Docs: TKT-WQ925LL'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the background-run binding in `bindBackgroundRequest`, the stale-skip rule in `applyProposal`, the retry split in `RunLLMAction` and `handleLLMActionJob`, `llmtask.Task.room`
- [x] Function/type docs if public API: `SetLLMActionQueue`, `NewLLMActionRunner`, `LLMActionRunner.RunLLMAction`, `appbuild.SetLLMActionRunner`, `appbuild.LLMActionQueue`, `appbuild.WithServerAIConfig`, `dataentryconfig.LLMCreateOutput`

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): rela-server `-ai-config` flag help

## User-facing Documentation

- [x] `GUIDE-data-entry.md` (generated to `docs/data-entry.md` with `just docs`): AI actions section, `outputs.create`, `max_outgoing` counting, server-wide provider, "Applying without review (`apply: auto`)" with list and flow use and the prompt-injection risk
- [x] `GUIDE-flows.md` (generated to `docs/flows.md`): job step `action` accepts an `apply: auto` llm action

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no new endpoint; `run_action` proposal gains `create`, documented in the guide)
