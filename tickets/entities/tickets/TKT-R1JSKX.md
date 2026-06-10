---
id: TKT-R1JSKX
type: ticket
title: 'Pinia Colada foundation: targeted SSE invalidation + KanbanView migration'
kind: enhancement
priority: high
effort: m
status: done
---

First slice of the query-cache migration (FEAT-XY2D1L):

- Install `@pinia/colada` and wire it in `main.ts`.
- Map SSE entity events to targeted query-key invalidation (`['entity', type, id]` and `['list', type]`) while keeping `entitiesStore.invalidateAll()` for unmigrated views.
- Migrate KanbanView from its local `entities` ref + `loading` flag + `cacheVersion` watch to `useQuery` — background refetch on invalidation, no more full-board spinner on every SSE event (including self-echoes from autosave).
- Drag-drop becomes a `useMutation` with copy-on-write optimistic update and rollback + toast on failure, replacing the in-place mutation of cached entity objects.

Scope deliberately excludes EntityList/EntityDetail/Dashboard/Search/Analyze
migrations and the eventual deletion of the entities-store TTL cache — those
follow view-by-view once this pattern is reviewed and merged.
