import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { defineComponent } from 'vue'
import KanbanView from './KanbanView.vue'
import RlBoard from 'rela-components/components/board/RlBoard.vue'
import type { Section } from 'rela-components/types'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import { withPageHeader } from '@/composables/pageHeaderTestHost'
import { styleColor } from '@/composables/useRelationColumns'
import type { Entity, ListResponse } from '@/types'

// A relation-backed board (`columns_from`, TKT-KJ3Q07): each column offers
// Add with the column's target prefilled, a drop re-points the card with a
// full linkage, and `style_from` colours each column by its target.

const listAllEntitiesMock = vi.fn()
const updateEntityMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
}))
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/kanban/bord' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

vi.mock('@/composables/useBackTarget', () => ({ useBackTarget: () => null }))

const ModalStub = defineComponent({
  name: 'InlineCreateFormModal',
  props: ['prefill', 'addAnother'],
  emits: ['created', 'created-another', 'close'],
  template: '<div class="modal-stub" />',
})

function page(data: Entity[], actions: Record<string, boolean> = {}): ListResponse<Entity> {
  return {
    data,
    meta: { total: data.length, page: 1, per_page: 100, has_more: false },
    included: {},
    _actions: actions,
  } as unknown as ListResponse<Entity>
}

const statuses: Entity[] = [
  { id: 'ST-1', type: 'status', properties: { titel: 'Open', volgorde: 1, categorie: 'open' } },
  { id: 'ST-2', type: 'status', properties: { titel: 'Bezig', volgorde: 2, categorie: 'actief' } },
  { id: 'ST-3', type: 'status', properties: { titel: 'Klaar', volgorde: 3 } },
] as unknown as Entity[]

const task = {
  id: 'TASK-1',
  type: 'taak',
  properties: { titel: 'Doe iets' },
  relations: { heeft_status: ['ST-1'] },
  _actions: { update: true },
} as unknown as Entity

function seed(styleFrom?: string) {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set('bord', {
    entity: 'taak',
    title: 'Bord',
    column_property: '',
    columns_from: { relation: 'heeft_status', order_by: 'volgorde', style_from: styleFrom },
    card: { title: 'titel' },
    create_form: 'taak',
  } as never)
  schemaStore.entityTypes.set('taak', {
    label: 'Taak',
    properties: { titel: { type: 'string' } },
  } as never)
  schemaStore.entityTypes.set('status', {
    label: 'Status',
    properties: {
      titel: { type: 'string' },
      volgorde: { type: 'integer' },
      categorie: { type: 'statuscategorie' },
    },
  } as never)
  schemaStore.customTypes.set('statuscategorie', {
    values: ['open', 'actief', 'gereed'],
  } as never)
  schemaStore.relationTypes.set('heeft_status', {
    label: 'heeft status',
    from: ['taak'],
    to: ['status'],
    max_outgoing: 1,
  } as never)
  schemaStore.styles = { statuscategorie: { open: 'badge-gray', actief: 'badge-blue' } }
  listAllEntitiesMock.mockImplementation((type: string) =>
    Promise.resolve(type === 'status' ? page(statuses) : page([task], { create: true }))
  )
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest('taak', 'taken')
  _setEntityPluralForTest('status', 'statussen')
  listAllEntitiesMock.mockReset()
  updateEntityMock.mockReset()
  updateEntityMock.mockResolvedValue({ ...task, relations: { heeft_status: ['ST-2'] } })
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountBoard(styleFrom?: string) {
  seed(styleFrom)
  const wrapper = mount(withPageHeader(KanbanView), {
    attrs: { id: 'bord' },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada], stubs: { InlineCreateFormModal: ModalStub } },
  })
  await flushPromises()
  return wrapper
}

function board(wrapper: Awaited<ReturnType<typeof mountBoard>>) {
  return wrapper.findComponent(RlBoard)
}

function boardProps(wrapper: Awaited<ReturnType<typeof mountBoard>>) {
  return board(wrapper).props() as unknown as { sections: Section[]; showAdd: boolean }
}

function sections(wrapper: Awaited<ReturnType<typeof mountBoard>>): Section[] {
  return boardProps(wrapper).sections
}

describe('KanbanView with columns_from', () => {
  it('offers Add in each column and prefills the column target', async () => {
    const wrapper = await mountBoard()
    expect(boardProps(wrapper).showAdd).toBe(true)

    const column = sections(wrapper).find((s) => s.id === 'ST-2')
    board(wrapper).vm.$emit('add', column)
    await flushPromises()

    const modal = wrapper.findComponent(ModalStub)
    expect(modal.props('prefill')).toEqual({
      properties: {},
      relations: { heeft_status: [{ id: 'ST-2', type: 'status' }] },
    })
    expect(modal.props('addAnother')).toBe(false)
  })

  it('opens New without a prefill', async () => {
    const wrapper = await mountBoard()
    const newButton = wrapper.findAll('button').find((b) => b.text() === 'New')
    await newButton?.trigger('click')
    await flushPromises()
    const modal = wrapper.findComponent(ModalStub)
    expect(modal.props('prefill')).toBeUndefined()
    expect(modal.props('addAnother')).toBe(true)
  })

  it('re-points a dropped card with a full linkage', async () => {
    const wrapper = await mountBoard()
    const [from, to] = [sections(wrapper)[0], sections(wrapper)[1]]
    board(wrapper).vm.$emit('move', { item: from.items[0], to })
    await flushPromises()

    expect(updateEntityMock).toHaveBeenCalledTimes(1)
    const body = updateEntityMock.mock.calls[0][2]
    expect(body).toEqual({
      relations: { heeft_status: { data: [{ type: 'status', id: 'ST-2' }] } },
    })
  })

  it('colours each column from the target style_from value', async () => {
    const wrapper = await mountBoard('categorie')
    const colors = Object.fromEntries(sections(wrapper).map((s) => [s.id, s.color]))
    expect(colors).toEqual({ 'ST-1': 'grey', 'ST-2': 'blue', 'ST-3': undefined })
  })

  it('leaves columns uncoloured without style_from', async () => {
    const wrapper = await mountBoard()
    expect(sections(wrapper).map((s) => s.color)).toEqual([undefined, undefined, undefined])
  })
})

describe('styleColor', () => {
  it('maps app style classes to status colours', () => {
    const styles = { open: 'badge-gray', wachten: 'badge-orange', in_review: 'badge-purple' }
    expect(styleColor(styles, 'open')).toBe('grey')
    expect(styleColor(styles, 'wachten')).toBe('amber')
    expect(styleColor(styles, 'In Review')).toBe('blue')
    expect(styleColor(styles, 'gereed')).toBeUndefined()
    expect(styleColor(styles, undefined)).toBeUndefined()
    expect(styleColor(undefined, 'open')).toBeUndefined()
  })
})
