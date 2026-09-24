---
id: DOCS-62Y0ZU
type: docs-checklist
title: 'Docs: related() incoming hops in query scopes'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (gate direction and face rules, fold guard, fail-closed paths)
- [x] Function/type docs if public API (ResolveTraversal, GateTraversal, TraversalQuery, UngatedTraversal, StringShaped, QueryScopeTraversalFieldErrors, QueryScopeFilter)

## Project Documentation

- [x] ~~README updated~~ (N/A: no README-level change)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern; the rules live in docs/metamodel.md and godoc)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository has no changelog file)
- [x] API docs updated: docs/metamodel.md "Filtering on related entities" documents related() in query_scopes, inverse IDs, limits, and the 422 query_scope_unsupported response
