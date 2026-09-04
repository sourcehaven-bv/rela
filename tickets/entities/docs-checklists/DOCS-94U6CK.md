---
id: DOCS-94U6CK
type: docs-checklist
title: 'Docs: Typed state references and the store contract (Step 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (the load-bearing godoc set was a design deliverable: entity.Face's two rules, AllStates' not-world-resolution line, the observer skips, the FOR SHARE race rationale, migration 0011's rebuild reasoning, cursor restart semantics, RelationScope's IsIdentity warning)
- [x] Function/type docs if public API (Face/codec, GetEntityState, EntityQuery.AllStates, RelationQuery.FromFace, RelationData.FromFace, Event.Face, EntityHeader.Face, RelationScope + accessors, analysis.StateFinding/CheckStates, storeutil error constructors)

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated~~ (deferred deliberately: a store-contract note is worth adding once worlds land and the shape is complete; noted in the plan)
- [x] Help text accurate (`rela analyze states` help string describes the findings)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo has no changelog file)
- [x] API docs updated: relation `scope:` documented in docs-project GUIDE-metamodel + regenerated docs/metamodel.md (options table row + dedicated section); the mcp metamodel JSON carries the additive Scope field (golden regenerated)
