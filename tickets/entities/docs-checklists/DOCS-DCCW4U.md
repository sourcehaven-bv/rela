---
id: DOCS-DCCW4U
type: docs-checklist
title: 'Docs: Enforce relation cardinality at write time and add an atomic replace operation'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API: doc comments on ReplaceRelations, CardinalityError, InvalidRelationError, RelationCreateError and the capacity helpers in internal/entitymanager/cardinality.go, and on the CalDAV move path in caldav_write.go

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change; the demo has its own examples/relation-status-demo/README.md)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## User-facing Documentation

- [x] `GUIDE-concepts.md`: cardinality is enforced on write, which API calls return 422, grandfathered data, unbounded bulk paths, re-point in one PATCH, store atomicity and CalDAV moves. Regenerated into `docs/concepts.md`.
- [x] `GUIDE-metamodel.md`: the `max_outgoing` and `max_incoming` rows say new edges over the bound are refused. Regenerated into `docs/metamodel.md`.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no new endpoint; the documented API behaviour is in GUIDE-concepts)
