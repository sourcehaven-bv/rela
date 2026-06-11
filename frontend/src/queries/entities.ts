// Query-key factory for entity data (Pinia Colada).
//
// Keys are hierarchical so SSE invalidation can target by prefix match:
//
//   ['entities']                      — every entity query
//   ['entities', type]                — all queries for one entity type
//   ['entities', type, 'list']       — the type's list queries
//   ['entities', type, 'detail', id] — a single entity
//
// useEvents maps SSE entity events ({type, id}) onto these prefixes;
// views subscribe via useQuery with the same keys, so an invalidation
// marks their query stale and triggers a background refetch while they
// are mounted — no spinner, no manual cacheVersion watching.
//
// This replaces the entities-store TTL cache view by view; see
// FEAT-XY2D1L. Unmigrated views still rely on entitiesStore.invalidateAll().
export const entityKeys = {
  root: ['entities'] as const,
  type: (type: string) => ['entities', type] as const,
  list: (type: string) => ['entities', type, 'list'] as const,
  detail: (type: string, id: string) => ['entities', type, 'detail', id] as const,
}
