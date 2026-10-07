---
id: DOCS-UVV3TV
type: docs-checklist
title: 'Docs: List group_by on a single-valued relation'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API: Go doc comments on the relation form of `ListGroupBy` in groupby.go and its validation

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change; the demo has its own examples/relation-status-demo/README.md)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## User-facing Documentation

- [x] `GUIDE-data-entry.md`: a list can group on a relation with `group_by: {relation, offered_by, order_by}`, sharing order and the Other section with the board. Regenerated into `docs/data-entry.md`.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no new endpoint; the documented API behaviour is in GUIDE-concepts)
