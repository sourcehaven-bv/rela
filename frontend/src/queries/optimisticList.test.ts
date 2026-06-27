import { describe, it, expect, vi } from 'vitest'
import {
  beginOptimistic,
  beginOptimisticRemove,
  rollbackOptimistic,
  settleOptimistic,
} from './optimisticList'
import type { Entity, ListResponse } from '@/types'

// A minimal in-memory stand-in for Pinia Colada's QueryCache: enough surface
// (get/set/cancel/invalidate keyed by a stringified key) to exercise the
// optimistic mechanics without the real runtime. The identity semantics that
// matter — getQueryData returns the same object reference setQueryData stored
// — are preserved, which is exactly what the rollback check depends on.
function makeFakeCache(initial?: ListResponse<Entity>) {
  const store = new Map<string, unknown>()
  const cancelled: string[] = []
  const invalidated: string[] = []
  const k = (key: readonly string[]) => key.join('|')
  if (initial) store.set(k(['entities', 'ticket', 'list']), initial)
  return {
    cancelled,
    invalidated,
    getQueryData: vi.fn((key: readonly string[]) => store.get(k(key))),
    setQueryData: vi.fn((key: readonly string[], data: unknown) => store.set(k(key), data)),
    cancelQueries: vi.fn(({ key }: { key: readonly string[] }) => cancelled.push(k(key))),
    invalidateQueries: vi.fn(({ key }: { key: readonly string[] }) => {
      invalidated.push(k(key))
      return Promise.resolve()
    }),
  }
}

const KEY = ['entities', 'ticket', 'list'] as const

function makeList(...ids: string[]): ListResponse<Entity> {
  return {
    data: ids.map((id) => ({ id, type: 'ticket', properties: { status: 'open' } })),
    meta: { total: ids.length, page: 1, per_page: 25, has_more: false },
    included: {},
  }
}

describe('optimisticList', () => {
  describe('beginOptimistic (update)', () => {
    it('cancels in-flight queries and applies a copy-on-write update', () => {
      const initial = makeList('T-1', 'T-2')
      const cache = makeFakeCache(initial)
      const ctx = beginOptimistic(cache as never, KEY, 'T-2', (e) => ({
        ...e,
        properties: { ...e.properties, status: 'done' },
      }))

      expect(cache.cancelled).toEqual(['entities|ticket|list'])
      expect(ctx.previous).toBe(initial)
      // The cached value is a new object (not mutated in place).
      expect(ctx.optimistic).not.toBe(initial)
      expect(cache.getQueryData(KEY)).toBe(ctx.optimistic)
      // Only the matching entity changed; the others are untouched refs.
      expect(ctx.optimistic!.data[1].properties.status).toBe('done')
      expect(ctx.optimistic!.data[0]).toBe(initial.data[0])
    })

    it('no-ops the cache write when the list was not cached', () => {
      const cache = makeFakeCache()
      const ctx = beginOptimistic(cache as never, KEY, 'T-1', (e) => e)
      expect(ctx.previous).toBeUndefined()
      expect(ctx.optimistic).toBeUndefined()
      expect(cache.setQueryData).not.toHaveBeenCalled()
    })
  })

  describe('beginOptimisticRemove (delete)', () => {
    it('drops the matching entity from the cached list', () => {
      const cache = makeFakeCache(makeList('T-1', 'T-2', 'T-3'))
      const ctx = beginOptimisticRemove(cache as never, KEY, 'T-2')
      expect(ctx.optimistic!.data.map((e) => e.id)).toEqual(['T-1', 'T-3'])
      expect(cache.getQueryData(KEY)).toBe(ctx.optimistic)
    })
  })

  // The three orderings RR-IVBO9K asked to pin.
  describe('settle / rollback orderings', () => {
    it('success: settle invalidates so the optimistic guess reconciles', async () => {
      const cache = makeFakeCache(makeList('T-1'))
      const ctx = beginOptimistic(cache as never, KEY, 'T-1', (e) => e)
      await settleOptimistic(cache as never, ctx)
      expect(cache.invalidated).toEqual(['entities|ticket|list'])
    })

    it('error with no intervening refetch: rollback restores previous', () => {
      const initial = makeList('T-1', 'T-2')
      const cache = makeFakeCache(initial)
      const ctx = beginOptimisticRemove(cache as never, KEY, 'T-2')
      // Cache currently holds the optimistic (T-2 removed) value.
      expect(cache.getQueryData(KEY)).toBe(ctx.optimistic)

      rollbackOptimistic(cache as never, ctx)
      // Restored to the original two-row list.
      expect(cache.getQueryData(KEY)).toBe(initial)
    })

    it('error after an intervening refetch: rollback is skipped', () => {
      const initial = makeList('T-1', 'T-2')
      const cache = makeFakeCache(initial)
      const ctx = beginOptimisticRemove(cache as never, KEY, 'T-2')

      // A background refetch resolves between onMutate and onError, writing
      // newer server truth into the cache.
      const fresh = makeList('T-1', 'T-2', 'T-9')
      cache.setQueryData(KEY, fresh)

      rollbackOptimistic(cache as never, ctx)
      // The newer value survives — rollback must NOT stomp it.
      expect(cache.getQueryData(KEY)).toBe(fresh)
    })

    it('rollback no-ops when nothing was written optimistically', () => {
      const cache = makeFakeCache()
      const ctx = beginOptimistic(cache as never, KEY, 'T-1', (e) => e)
      rollbackOptimistic(cache as never, ctx)
      expect(cache.setQueryData).not.toHaveBeenCalled()
    })
  })
})
