---
id: DOCS-I9U01U
type: docs-checklist
title: 'Docs: escape feed text (TKT-343IKH)'
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (ValueKind semantics, link encoding, plainHost, append refusal)
- [x] Function/type docs if public API (ingest.ValueKind, ingest.EscapeMarkdown, Source.Kind)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting rule)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (docs/feed-subscriptions.md template values and Trust; docs/webhooks.md states values are not escaped)
