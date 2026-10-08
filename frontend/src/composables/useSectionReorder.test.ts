import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'
import { useSectionReorder } from './useSectionReorder'
import { useUIStore } from '@/stores/ui'
import type { ViewResponse, ViewSection } from '@/api/views'
import type { RelationOrder } from '@/types'

const moveRelationMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  moveRelation: (...args: unknown[]) => moveRelationMock(...args),
}))

const order: RelationOrder = { relation: 'has-step', anchor: 'REC-1', anchor_type: 'recipe', movable: true }

function section(over: Partial<ViewSection> = {}): ViewSection {
  return {
    sectionId: 'steps',
    display: 'table',
    rows: ['a', 'b', 'c'].map((id) => ({ entityId: id, entityType: 'step', cells: [] })),
    relationOrder: order,
    ...over,
  } as ViewSection
}

function setup(sec: ViewSection = section()) {
  setActivePinia(createPinia())
  const view = ref<ViewResponse | null>({ entry: { id: 'REC-1' }, sections: [sec] } as ViewResponse)
  const reload = vi.fn().mockResolvedValue(undefined)
  const api = useSectionReorder({ view, reload })
  const ids = () => view.value?.sections[0].rows?.map((r) => r.entityId)
  return { api, ids, reload }
}

beforeEach(() => {
  moveRelationMock.mockReset().mockResolvedValue(undefined)
})

describe('useSectionReorder', () => {
  it('shows the move at once, sends it, and reloads the view', async () => {
    const { api, ids, reload } = setup()
    let release!: () => void
    moveRelationMock.mockImplementation(() => new Promise<void>((r) => (release = r)))
    const done = api.onReorder(0, { itemId: 'c', targetId: 'a', placement: 'before' })
    await flushPromises()
    expect(ids()).toEqual(['c', 'a', 'b'])
    release()
    await done
    expect(moveRelationMock).toHaveBeenCalledWith('recipe', 'REC-1', 'has-step', 'c', { before: 'a' })
    expect(reload).toHaveBeenCalled()
  })

  it('reports a failed move and reloads to undo it', async () => {
    moveRelationMock.mockRejectedValue(new Error('nope'))
    const { api, reload } = setup()
    const error = vi.spyOn(useUIStore(), 'error')
    await api.onReorder(0, { itemId: 'a', step: 1 })
    expect(error).toHaveBeenCalled()
    expect(reload).toHaveBeenCalled()
  })

  it('announces a keyboard move by row label in its own section only', async () => {
    const sec = section()
    sec.rows![0].cells = [{ values: ['Alpha step'] }]
    const { api } = setup(sec)
    await api.onReorder(0, { itemId: 'a', step: 1 })
    expect(api.statusOf(0)).toBe('Alpha step moved down')
    expect(api.statusOf(1)).toBe('')
  })

  it('reloads once after moves made in quick succession', async () => {
    const { api, ids, reload } = setup()
    const done = [
      api.onReorder(0, { itemId: 'c', step: -1 }),
      api.onReorder(0, { itemId: 'c', step: -1 }),
    ]
    expect(ids()).toEqual(['c', 'a', 'b'])
    await Promise.all(done)
    expect(moveRelationMock.mock.calls.map((c) => c[4])).toEqual([{ before: 'b' }, { before: 'a' }])
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['the principal may not move rows', section({ relationOrder: { ...order, movable: false } })],
    ['the section is not in relation order', section({ relationOrder: undefined })],
    ['the section is grouped', section({ isGrouped: true })],
    ['the section is not a table', section({ display: 'cards' } as Partial<ViewSection>)],
  ])('moves nothing when %s', async (_name, sec) => {
    const { api, ids } = setup(sec)
    expect(api.movable(sec)).toBe(false)
    await api.onReorder(0, { itemId: 'c', targetId: 'a', placement: 'before' })
    expect(moveRelationMock).not.toHaveBeenCalled()
    expect(ids()).toEqual(['a', 'b', 'c'])
  })
})
