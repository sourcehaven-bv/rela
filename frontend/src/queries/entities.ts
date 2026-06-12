import type { ListParams } from '@/types'

// Query-key factory for entity data (Pinia Colada).
//
// Keys are hierarchical so SSE invalidation can target by prefix match:
//
//   ['entities']                         — every entity query
//   ['entities', type]                   — all queries for one entity type
//   ['entities', type, 'list']          — the type's list queries (all params)
//   ['entities', type, 'list', '<params>'] — one parameterized list query
//   ['entities', type, 'detail', id]    — a single entity
//
// useEvents maps SSE entity events ({type, id}) onto these prefixes;
// views subscribe via useQuery with the same keys, so an invalidation
// marks their query stale and triggers a background refetch while they
// are mounted — no spinner, no manual cacheVersion watching.
//
// This replaces the entities-store TTL cache view by view; see
// FEAT-XY2D1L. Unmigrated views still rely on entitiesStore.invalidateAll().

// canonicalListParams produces a stable string for a list query's params,
// so two callers building the same filters/sort/page in different property
// order resolve to the same cache entry (the param-order cache-key bug from
// the frontend review). undefined/empty values are dropped.
export function canonicalListParams(params?: ListParams): string {
  if (!params) return ''
  const record = params as Record<string, unknown>
  const pairs = Object.keys(record)
    .sort()
    .filter((k) => record[k] !== undefined && record[k] !== '')
    .map((k) => `${k}=${String(record[k])}`)
  return pairs.join('&')
}

export const entityKeys = {
  root: ['entities'] as const,
  type: (type: string) => ['entities', type] as const,
  // Base list key (prefix) — invalidating this hits every parameter
  // variant for the type. Kanban uses this param-free form (one board =
  // one list query).
  list: (type: string) => ['entities', type, 'list'] as const,
  // Parameterized list key — one cache entry per distinct param set.
  // EntityList keys on this so page/filter/sort/search each cache apart
  // while still sharing the `list(type)` invalidation prefix.
  listParams: (type: string, params?: ListParams) =>
    ['entities', type, 'list', canonicalListParams(params)] as const,
  detail: (type: string, id: string) => ['entities', type, 'detail', id] as const,
}
