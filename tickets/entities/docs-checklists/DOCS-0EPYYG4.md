---
id: DOCS-0EPYYG4
type: docs-checklist
title: 'Docs: Gated paged reads for MCP list_entities and search with no text'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: ListEntityPage (gate in the store, Props refusal), readPage, listAdmitted (family batching), typePages (growing pages), the store.go cursor contract
- [x] Function/type docs if public API: PageQuery, EntityPage, ErrPropsNotPushable, ListEntityPage on each reader

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing pushdown and collection-reads rules)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (if applicable): GUIDE-mcp-server states the list_entities order (byte-wise by id, then face) and the 1000 limit cap; docs/mcp-server.md regenerated; the tool descriptions say the same
