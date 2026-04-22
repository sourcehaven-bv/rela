---
id: TKT-E1FO1
type: ticket
title: Add rela.document.depends_on(id) for SSE dependency tracking in Lua doc scripts
kind: enhancement
priority: low
status: backlog
---

## Problem

Lua document scripts (introduced in TKT-CGBVW) can compose markdown from many
entities, but the data-entry frontend only knows to reload the document when the
*entry* entity changes. If the script walked 20 related entities and one of them
changes, the rendered panel goes stale until the user hits refresh.

## Proposal

Add a Lua binding that lets a document script declare the entity IDs it depends
on:

```lua
rela.document.depends_on(entity.id)
```

The render response collects these into `entity_ids` (same shape the existing
`DocumentResult.Entities` uses). The frontend's `DocumentsPanel.vue` already
tracks these IDs and triggers a reload on matching SSE entity-change events — no
frontend work needed.

## Alternative considered

Implicit tracking by intercepting `rela.get_entity` / `rela.list_entities` /
`rela.trace_*`. Rejected for V1 because it requires wrapping every read binding
and has surprising behavior (a throwaway `list_entities` during exploration
would register unwanted deps). Explicit opt-in is more predictable.

## Scope

- Add `depends_on(id)` to `rela.document` table (only present in document mode).
- Accumulate IDs into the document render context.
- Return them alongside the rendered HTML as `entity_ids`.
- Update docs in GUIDE-data-entry.

## Out of scope

- Automatic tracking.
- Cross-document dependency propagation.
