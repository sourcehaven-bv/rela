import type { QueryCache } from '@pinia/colada'
import type { Entity, ListResponse } from '@/types'

// Optimistic-update cache mechanics for entity-list mutations, shared by
// KanbanView's drag-drop and EntityList's delete (FEAT-XY2D1L, RR-IVBO9K).
//
// The three steps below are the cache half of a Pinia Colada useMutation:
//   onMutate  → beginOptimistic(...)  (cancel + copy-on-write + snapshot)
//   onError   → rollbackOptimistic(...) (revert iff still our value)
//   onSettled → settleOptimistic(...)  (invalidate → reconcile with server)
//
// Pulling them out of the component keeps the rollback identity check (the
// subtle bit — see below) in one unit-tested place instead of copied per
// view. Cached list objects are never mutated in place; every change is a
// shallow copy so other subscribers holding references are unaffected.

export interface OptimisticListContext {
  key: readonly string[]
  // The pre-mutation cache value, restored on rollback. undefined when the
  // list wasn't cached yet (nothing to roll back to).
  previous: ListResponse<Entity> | undefined
  // The exact object we wrote optimistically. Identity-compared on rollback
  // so a background refetch that landed mid-mutation is not clobbered.
  optimistic: ListResponse<Entity> | undefined
}

// beginOptimistic cancels any in-flight refetch (so it can't overwrite our
// optimistic write), reads the current list, applies `update` to each
// matching entity as a copy-on-write, writes it back, and returns the
// context the error/settle steps need.
export function beginOptimistic(
  queryCache: QueryCache,
  key: readonly string[],
  matchId: string,
  update: (entity: Entity) => Entity
): OptimisticListContext {
  queryCache.cancelQueries({ key })
  const previous = queryCache.getQueryData<ListResponse<Entity>>(key)
  const optimistic = previous && {
    ...previous,
    data: previous.data.map((e) => (e.id === matchId ? update(e) : e)),
  }
  if (optimistic) queryCache.setQueryData(key, optimistic)
  return { key, previous, optimistic }
}

// beginOptimisticRemove is the delete counterpart: drops the matching
// entity from the cached list (copy-on-write) so the row disappears
// immediately, returning the same context shape for rollback/settle.
export function beginOptimisticRemove(
  queryCache: QueryCache,
  key: readonly string[],
  matchId: string
): OptimisticListContext {
  queryCache.cancelQueries({ key })
  const previous = queryCache.getQueryData<ListResponse<Entity>>(key)
  const optimistic = previous && {
    ...previous,
    data: previous.data.filter((e) => e.id !== matchId),
  }
  if (optimistic) queryCache.setQueryData(key, optimistic)
  return { key, previous, optimistic }
}

// rollbackOptimistic restores `previous` — but ONLY if the cache still holds
// the exact object we wrote. If a refetch resolved between onMutate and
// onError, the cache holds newer server truth that we must not stomp.
//
// Accepts a Partial because Pinia Colada types a mutation's onError context
// as partial: onMutate may not have run if the mutation rejected before it.
export function rollbackOptimistic(
  queryCache: QueryCache,
  ctx: Partial<OptimisticListContext>
): void {
  if (!ctx.key || !ctx.optimistic) return
  if (queryCache.getQueryData(ctx.key) === ctx.optimistic) {
    queryCache.setQueryData(ctx.key, ctx.previous)
  }
}

// settleOptimistic invalidates the list so it refetches and reconciles with
// the server (on success this replaces the optimistic guess with the real
// row; on a rolled-back error it confirms the reverted state). Awaitable so
// the mutation can stay "loading" until the refetch lands.
export function settleOptimistic(
  queryCache: QueryCache,
  ctx: Partial<OptimisticListContext>
): Promise<unknown> {
  if (!ctx.key) return Promise.resolve()
  return queryCache.invalidateQueries({ key: ctx.key })
}
