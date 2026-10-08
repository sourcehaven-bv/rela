import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada, useQueryCache } from '@pinia/colada'
import { defineComponent, ref } from 'vue'
import { useListReorder } from './useListReorder'
import { useUIStore } from '@/stores/ui'
import type { Entity, ListResponse, RelationOrder } from '@/types'

const moveRelationMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  moveRelation: (...args: unknown[]) => moveRelationMock(...args),
}))

const key = ['entities', 'task', 'list', 'p1'] as const
const order: RelationOrder = { relation: 'has-task', anchor: 'PRJ-1', anchor_type: 'project', movable: true }
const row = (id: string) => ({ id, type: 'task', properties: {} }) as unknown as Entity

function setup(opts: { order?: RelationOrder; active?: boolean } = {}) {
  const pinia = createPinia()
  setActivePinia(pinia)
  let api!: ReturnType<typeof useListReorder>
  let cache!: ReturnType<typeof useQueryCache>
  const active = ref(opts.active ?? true)
  mount(
    defineComponent({
      setup() {
        cache = useQueryCache()
        cache.setQueryData(key, { data: ['a', 'b', 'c'].map(row), meta: {} } as ListResponse<Entity>)
        api = useListReorder({
          order: () => opts.order ?? order,
          active: () => active.value,
          rows: () => cache.getQueryData<ListResponse<Entity>>(key)?.data ?? [],
          key: () => key,
          type: () => 'task',
        })
        return () => null
      },
    }),
    { global: { plugins: [pinia, PiniaColada] } },
  )
  const ids = () => cache.getQueryData<ListResponse<Entity>>(key)?.data.map((e) => e.id)
  return { api, ids, active, cache }
}

beforeEach(() => {
  moveRelationMock.mockReset().mockResolvedValue(undefined)
})

describe('useListReorder', () => {
  it('shows the move at once and sends it to the anchor', async () => {
    const { api, ids } = setup()
    const done = api.onReorder({ itemId: 'c', targetId: 'a', placement: 'before' })
    await flushPromises()
    expect(ids()).toEqual(['c', 'a', 'b'])
    expect(moveRelationMock).toHaveBeenCalledWith('project', 'PRJ-1', 'has-task', 'c', { before: 'a' })
    await done
  })

  it('puts the rows back and reports a failed move', async () => {
    moveRelationMock.mockRejectedValue(new Error('nope'))
    const { api, ids } = setup()
    const error = vi.spyOn(useUIStore(), 'error')
    await api.onReorder({ itemId: 'a', step: 1 })
    expect(ids()).toEqual(['a', 'b', 'c'])
    expect(error).toHaveBeenCalled()
  })

  it('keeps moving after a refetch fails', async () => {
    const { api, ids, cache } = setup()
    const refetch = vi.spyOn(cache, 'invalidateQueries').mockRejectedValue(new Error('aborted'))
    await api.onReorder({ itemId: 'c', step: -1 })
    expect(ids()).toEqual(['a', 'c', 'b'])
    await api.onReorder({ itemId: 'c', step: -1 })
    expect(refetch).toHaveBeenCalledTimes(2)
    expect(moveRelationMock).toHaveBeenCalledTimes(2)
  })

  it('shows the move before the call returns', () => {
    const { api, ids } = setup()
    void api.onReorder({ itemId: 'c', step: -1 })
    expect(ids()).toEqual(['a', 'c', 'b'])
  })

  it('sends moves one at a time, each planned on the last', async () => {
    let release!: () => void
    moveRelationMock.mockImplementationOnce(() => new Promise<void>((r) => (release = r)))
    const { api } = setup()
    const first = api.onReorder({ itemId: 'c', step: -1 })
    const second = api.onReorder({ itemId: 'c', step: -1 })
    await flushPromises()
    expect(moveRelationMock).toHaveBeenCalledTimes(1)
    release()
    await Promise.all([first, second])
    expect(moveRelationMock.mock.calls.map((c) => c[4])).toEqual([{ before: 'b' }, { before: 'a' }])
  })

  it.each([
    ['the principal may not move rows', { order: { ...order, movable: false } }],
    ['the reader sorted the list', { active: false }],
  ])('moves nothing when %s', async (_name, opts) => {
    const { api, ids } = setup(opts)
    expect(api.reorderable.value).toBe(false)
    await api.onReorder({ itemId: 'c', targetId: 'a', placement: 'before' })
    expect(moveRelationMock).not.toHaveBeenCalled()
    expect(ids()).toEqual(['a', 'b', 'c'])
  })
})
