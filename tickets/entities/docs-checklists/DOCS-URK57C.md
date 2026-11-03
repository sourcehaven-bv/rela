---
id: DOCS-URK57C
type: docs-checklist
title: 'Docs: Create a linked entity from a markdown task item'
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the README lists no schema keys)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting pattern; the converter follows the existing CreateLinked and copy-visibility rules)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable)

**Evidence:**

- `docs-project/entities/guides/GUIDE-metamodel.md` documents the
`task_item_conversions:` key, the item modes, the load-time refusals and the
checks. `GUIDE-data-entry.md` documents the endpoints, the offers field,
warnings and the error table, including `entity_id` on `conversion_incomplete`.
`just docs` regenerated `docs/metamodel.md` and `docs/data-entry.md`.
- The generated OpenAPI documents both endpoints, `warnings`, the 500
`conversion_incomplete` response and `entity_id`.
