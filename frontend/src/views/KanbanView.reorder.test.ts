import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import { usePageStore } from '@/stores/pages'
import { _setEntityPluralForTest } from '@/api/entities'
import { withPageHeader } from '@/composables/pageHeaderTestHost'
import type { Entity, ListResponse, RelationOrder } from '@/types'

// A board on a tab shown in relation order lets the reader set the card
// order: a drop against a card is sent as a move of the anchor's edge.

const listAllEntitiesMock = vi.fn()
const updateEntityMock = vi.fn()
const moveRelationMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
}))
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
  moveRelation: (...args: unknown[]) => moveRelationMock(...args),
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/p/topic/TOP-1/board' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))
vi.mock('@/composables/useBackTarget', () => ({ useBackTarget: () => null }))

const order: RelationOrder = { relation: 'bestaat_uit', anchor: 'TOP-1', anchor_type: 'topic', movable: true }
const card = (id: string, status: string) =>
  ({ id, type: 'taak', properties: { title: id, status }, _actions: { update: true } }) as unknown as Entity

function seed(opts: { orderable: boolean; relationOrder?: RelationOrder }) {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set('taken_bord', {
    entity: 'taak',
    title: 'Taken',
    column_property: 'status',
    columns: [{ value: 'todo', label: 'Te doen' }, { value: 'done', label: 'Klaar' }],
    card: { title: 'title' },
  } as never)
  schemaStore.entityTypes.set('taak', {
    label: 'Taak',
    properties: { title: { type: 'string' }, status: { type: 'enum', values: ['todo', 'done'] } },
  } as never)
  schemaStore.relationTypes.set('bestaat_uit', {
    label: 'bestaat uit', from: ['topic'], to: ['taak'],
    ...(opts.orderable ? { orderable: { outgoing: true } } : {}),
  })
  usePageStore().set({
    topic: {
      label: 'Topic',
      entity_type: 'topic',
      tabs: [{
        id: 'board', label: 'Board', view: 'kanban', target: 'taken_bord',
        links: [{ type: 'taak', relation: 'bestaat_uit', direction: 'outgoing' }],
      }],
    } as never,
  })
  listAllEntitiesMock.mockResolvedValue({
    data: [card('A', 'todo'), card('B', 'todo'), card('C', 'done')],
    meta: { total: 3, page: 1, per_page: 100, has_more: false, relation_order: opts.relationOrder },
    included: {},
    _actions: { create: true },
  } as unknown as ListResponse<Entity>)
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest('taak', 'taken')
  _setEntityPluralForTest('topic', 'topics')
  listAllEntitiesMock.mockReset()
  updateEntityMock.mockReset().mockResolvedValue({})
  moveRelationMock.mockReset().mockResolvedValue(undefined)
})
afterEach(() => {
  document.body.innerHTML = ''
})

async function mountBoard(opts: { orderable: boolean; relationOrder?: RelationOrder }) {
  seed(opts)
  const wrapper = mount(withPageHeader(KanbanView), {
    attrs: { id: 'taken_bord', pageScope: { page: 'topic', tab: 'board', entity: 'TOP-1' } },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  const board = wrapper.findComponent({ name: 'RlBoard' })
  const [todo, done] = board.props('sections') as { items: unknown[] }[]
  return { board, todo, done }
}

describe('KanbanView card order', () => {
  it('moves a card within its column by moving the anchor’s edge', async () => {
    const { board, todo } = await mountBoard({ orderable: true, relationOrder: order })
    expect(board.props('reorder')).toBe(true)

    board.vm.$emit('move', { item: todo.items[1], to: todo, at: { targetId: 'A', placement: 'before' } })
    await flushPromises()

    expect(updateEntityMock).not.toHaveBeenCalled()
    expect(moveRelationMock).toHaveBeenCalledWith('topic', 'TOP-1', 'bestaat_uit', 'B', { before: 'A' })
  })

  it('writes the column first, then the place, for a drop into another column', async () => {
    const { board, todo, done } = await mountBoard({ orderable: true, relationOrder: order })
    updateEntityMock.mockImplementation(() => {
      expect(moveRelationMock).not.toHaveBeenCalled()
      return Promise.resolve({})
    })

    board.vm.$emit('move', { item: todo.items[0], to: done, at: { targetId: 'C', placement: 'after' } })
    await flushPromises()

    expect(updateEntityMock.mock.calls[0][2]).toMatchObject({ properties: { status: 'done' } })
    expect(moveRelationMock).toHaveBeenCalledWith('topic', 'TOP-1', 'bestaat_uit', 'A', { after: 'C' })
  })

  it('does not place a card whose column change failed', async () => {
    updateEntityMock.mockRejectedValue(new Error('denied'))
    const { board, todo, done } = await mountBoard({ orderable: true, relationOrder: order })

    board.vm.$emit('move', { item: todo.items[0], to: done, at: { targetId: 'C', placement: 'after' } })
    await flushPromises()

    expect(moveRelationMock).not.toHaveBeenCalled()
  })

  it('refuses out loud to move a card it may not update into another column', async () => {
    const { board, todo, done } = await mountBoard({ orderable: true, relationOrder: order })
    const error = vi.spyOn(useUIStore(), 'error')
    const own = todo.items[0] as { entity: Entity }
    const item = { ...own, entity: { ...own.entity, _actions: { update: false } } }

    board.vm.$emit('move', { item, to: done, at: { targetId: 'C', placement: 'after' } })
    await flushPromises()

    expect(error).toHaveBeenCalled()
    expect(updateEntityMock).not.toHaveBeenCalled()
    expect(moveRelationMock).not.toHaveBeenCalled()
  })

  it.each([
    ['the relation is not orderable', { orderable: false, relationOrder: undefined }],
    ['the principal may not move cards', { orderable: true, relationOrder: { ...order, movable: false } }],
  ])('offers no card order when %s', async (_name, opts) => {
    const { board } = await mountBoard(opts)
    expect(board.props('reorder')).toBe(false)
  })
})
