import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada, useQueryCache } from '@pinia/colada'
import { defineComponent } from 'vue'
import { replaceRun, useManualReorder } from './useManualReorder'
import { useUIStore } from '@/stores/ui'
import type { Entity, ListResponse } from '@/types'

const moveListRowMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  moveListRow: (...args: unknown[]) => moveListRowMock(...args),
}))

const key = ['entities', 'task', 'list', 'p1'] as const
const row = (id: string, group = 'x') =>
  ({ id, type: 'task', properties: { g: group } }) as unknown as Entity
const params = { list_id: 'todo', manual_order: 1 as const, page: 2 }

function setup(grouped = false) {
  const pinia = createPinia()
  setActivePinia(pinia)
  let api!: ReturnType<typeof useManualReorder>
  let cache!: ReturnType<typeof useQueryCache>
  mount(
    defineComponent({
      setup() {
        cache = useQueryCache()
        cache.setQueryData(key, {
          data: [row('a'), row('b'), row('c', 'y'), row('d', 'y')],
          meta: {},
        } as ListResponse<Entity>)
        const rows = () => cache.getQueryData<ListResponse<Entity>>(key)?.data ?? []
        api = useManualReorder({
          active: () => true,
          rows,
          sections: () =>
            grouped
              ? [
                  { items: rows().filter((r) => r.properties.g === 'x') },
                  { items: rows().filter((r) => r.properties.g === 'y') },
                ]
              : [{ items: rows() }],
          params: () => params,
          key: () => key,
          type: () => 'task',
        })
        return () => null
      },
    }),
    { global: { plugins: [pinia, PiniaColada] } }
  )
  const ids = () => cache.getQueryData<ListResponse<Entity>>(key)?.data.map((e) => e.id)
  return { api, ids }
}

beforeEach(() => {
  moveListRowMock.mockReset().mockResolvedValue(undefined)
})

describe('useManualReorder', () => {
  it('shows the move at once and sends it with the list params', async () => {
    const { api, ids } = setup()
    const done = api.onReorder({ itemId: 'c', targetId: 'a', placement: 'before' })
    expect(ids()).toEqual(['c', 'a', 'b', 'd'])
    await done
    expect(moveListRowMock).toHaveBeenCalledWith(params, 'c', { before: 'a' })
  })

  it('moves inside a section and refuses a move into another', async () => {
    const { api, ids } = setup(true)
    await api.onReorder({ itemId: 'd', targetId: 'c', placement: 'before' })
    expect(ids()).toEqual(['a', 'b', 'd', 'c'])
    moveListRowMock.mockClear()
    await api.onReorder({ itemId: 'a', targetId: 'd', placement: 'after' })
    expect(moveListRowMock).not.toHaveBeenCalled()
  })

  it('ignores a step past the edge', async () => {
    const { api } = setup()
    await api.onReorder({ itemId: 'a', step: -1 })
    expect(moveListRowMock).not.toHaveBeenCalled()
  })

  it('puts the rows back when the move fails', async () => {
    const { api, ids } = setup()
    moveListRowMock.mockRejectedValue(new Error('boom'))
    const ui = useUIStore()
    const error = vi.spyOn(ui, 'error')
    await api.onReorder({ itemId: 'b', step: 1 })
    await flushPromises()
    expect(ids()).toEqual(['a', 'b', 'c', 'd'])
    expect(error).toHaveBeenCalled()
  })
})

describe('replaceRun', () => {
  it('replaces the run in place', () => {
    const all = ['a', 'b', 'c', 'd'].map((id) => row(id))
    const next = replaceRun(all, [all[1], all[2]], [all[2], all[1]])
    expect(next.map((r) => r.id)).toEqual(['a', 'c', 'b', 'd'])
  })
})
