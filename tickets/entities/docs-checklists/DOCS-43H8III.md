---
id: DOCS-43H8III
type: docs-checklist
title: 'Docs: Reject type-disallowed relation edges on every write surface'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (checkRelationEdges order, create pre-check ACL guard, endpointTypeRefusal)
- [x] Function/type docs if public API (metamodel.RelationEndCode, the two codes, InvalidRelationError.Code)

## Project Documentation

- [x] ~~README updated~~ (N/A: no README content covers relation validation)
- [x] CLAUDE.md updated (if new patterns) (internal/dataentry/CLAUDE.md validation table and the hard-rule paragraph)
- [x] ~~Help text accurate~~ (N/A: no CLI flag or help text changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PRs)
- [x] API docs updated (docs/data-entry/api-reference.md: hard 422 bullet, id field row, warnings table; DEC-0D8UZX3 and DEC-HWZHA)
