---
id: TKT-24QVHB
type: ticket
title: EntityList migration to Pinia Colada (FEAT-XY2D1L slice 2)
kind: enhancement
priority: high
effort: m
status: done
---

Second slice of the query-cache migration (FEAT-XY2D1L), after the KanbanView
proof-of-pattern (#953).

**EntityList migration**
- Extend `entityKeys.list(type, params?)` with a canonicalized (sorted-key) params dimension so EntityList's per-page/filter/sort/search queries each key distinctly; Kanban's existing param-free `list(type)` call stays valid. Also fixes the param-order-sensitive cache-key bug from the review.
- Split local `page` (input → query key) from `meta` (output ← query data) to break the read/write cycle the old `meta.value.page` had.
- Replace `fetchGeneration` + `scheduleFetch`/`fetchPending` microtask coalescing + manual `loading` + `loadEntities` with one `useQuery` keyed on `queryParams`, `placeholderData: previous` to keep rows visible across param changes. SSE liveness comes free from the `['entities', <type>]` invalidation wired in #953 — EntityList previously never reacted to SSE at all.
- Delete becomes a `useMutation`; list load errors use `getErrorMessage` + inline error state (consistent with migrated Kanban).

**Shared optimistic helper (closes RR-IVBO9K)**
- Extract `src/queries/optimisticList.ts`: cancel → copy-on-write set → rollback-with-identity-check → invalidate-on-settle, unit-tested for the three orderings (success / rollback / refetch-superseded-rollback). Refactor KanbanView's `moveCard` onto it.

**B1a folded in**
- `api/entities.ts` gets a plural registry populated by the schema store; delete the `useSchemaStore` import from the API layer (layering inversion); `getPlural` throws on unknown type instead of fabricating a URL.

Scope excludes EntityDetail / Dashboard / Search / Analyze and the eventual
deletion of the entities-store TTL cache.
