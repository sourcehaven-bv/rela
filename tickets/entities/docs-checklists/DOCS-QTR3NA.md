---
id: DOCS-QTR3NA
type: docs-checklist
title: 'Docs: presence backend (TKT-MHXH87)'
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (lock order, round commit under h.mu, watching/joined/left, pending reschedule)
- [x] Function/type docs if public API (presence.Tracker, v1 presence types, OpenAPI)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: package is self-documenting; no new rule)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (api-reference: _presence, live semantics, _lease on view entries; GUIDE-acl-security presence section regenerated)
