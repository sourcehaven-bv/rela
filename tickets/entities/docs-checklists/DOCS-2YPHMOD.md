---
id: DOCS-2YPHMOD
type: docs-checklist
title: 'Docs: TKT-4USH1HO'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why free-text proposals start unticked, why the candidate read uses gated headers and redacts once, why the provider is skipped when nothing can be proposed, why only the error kind is logged
- [x] Function/type docs if public API: godoc on the `llmtask` package, `Task`, `Input`, `Proposal`, `MaxValueChars` and the `dataentryconfig` llm types; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: the feature follows existing rules for reads and writes)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md` "AI actions" section (configuration, schema-checked answers, value cap, limit total, NopACL note, prompt-injection and unticked text, stale and dropped proposals), plus `GUIDE-lua-scripting.md` and `GUIDE-mcp-server.md` (`run_action` proposal); regenerated with `just docs`
- [x] Security guide: the guide section names the prompt-injection risk and that `permission:` admits everyone without `acl.yaml`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: the action response gains `proposal`, documented in the guides; no OpenAPI route change)
