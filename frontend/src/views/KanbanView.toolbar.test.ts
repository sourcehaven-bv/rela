import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import SearchBox from '@/components/lists/SearchBox.vue'
import SortMenu from '@/components/lists/SortMenu.vue'
import { withPageHeader } from '@/composables/pageHeaderTestHost'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// A board has the list's toolbar: a search box and a Sort menu. Both are
// server-side: the free-text query and the sort ride on the board's list read
// (`q`, `sort`), so the board shows only the cards the server returns.

const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: vi.fn().mockResolvedValue(undefined),
}))

const mockRoute: { query: Record<string, unknown>; path: string } = { query: {}, path: '/kanban/board' }
const routerReplace = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: routerReplace }),
  useRoute: () => mockRoute,
  RouterLink: { props: ['to'], template: '<a :data-to="JSON.stringify(to)"><slot /></a>' },
}))

vi.mock('@/composables/useBackTarget', () => ({ useBackTarget: () => null }))

const KANBAN_ID = 'board'
const ENTITY_TYPE = 'taak'

function seedSchema() {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set(KANBAN_ID, {
    entity: ENTITY_TYPE,
    title: 'Taken',
    column_property: 'status',
    columns: [{ value: 'todo', label: 'Te doen' }],
    card: { title: 'title' },
  } as never)
  schemaStore.entityTypes.set(ENTITY_TYPE, {
    name: ENTITY_TYPE,
    label: 'Taak',
    properties: {
      title: { type: 'string' },
      status: { type: 'enum', values: ['todo'] },
      due: { type: 'date' },
      tags: { type: 'string', list: true },
    },
  } as never)
}

function card(id: string, title: string): Entity {
  return {
    id,
    type: ENTITY_TYPE,
    properties: { title, status: 'todo' },
    relations: {},
    _actions: { update: true, create: true },
  } as Entity
}

async function mountBoard() {
  seedSchema()
  listAllEntitiesMock.mockResolvedValue({
    data: [card('TAAK-1', 'Werk van Anna')],
    meta: { total: 1, page: 1, per_page: 100, has_more: false },
    included: {},
    _actions: { create: true },
  } as unknown as ListResponse<Entity>)
  const wrapper = mount(withPageHeader(KanbanView), {
    props: { id: KANBAN_ID },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

const lastParams = () => {
  const calls = listAllEntitiesMock.mock.calls
  return calls[calls.length - 1][1] as Record<string, unknown> | undefined
}

let pinia: Pinia
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest(ENTITY_TYPE, 'taken')
  listAllEntitiesMock.mockReset()
  routerReplace.mockReset()
  mockRoute.query = {}
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('KanbanView toolbar', () => {
  it('reads the search from ?q= and sends it with the board read', async () => {
    mockRoute.query = { q: 'anna' }
    const wrapper = await mountBoard()

    expect(lastParams()?.q).toBe('anna')
    expect(wrapper.find<HTMLInputElement>('.search-box input').element.value).toBe('anna')
    wrapper.unmount()
  })

  it('writes a typed search to the URL', async () => {
    const wrapper = await mountBoard()

    wrapper.findComponent(SearchBox).vm.$emit('update:modelValue', 'werk')
    await flushPromises()

    expect(routerReplace).toHaveBeenCalledWith({ query: { q: 'werk' } })
    wrapper.unmount()
  })

  it('sends no q and no sort by default', async () => {
    const wrapper = await mountBoard()

    expect(lastParams()?.q).toBeUndefined()
    expect(lastParams()?.sort).toBeUndefined()
    wrapper.unmount()
  })

  it('offers the entity properties but not a list-valued one', async () => {
    const wrapper = await mountBoard()

    const options = wrapper.findComponent(SortMenu).props('options') as { property: string }[]
    expect(options.map((o) => o.property)).toEqual(['title', 'status', 'due'])
    wrapper.unmount()
  })

  it('sends the chosen sort with the board read', async () => {
    const wrapper = await mountBoard()

    wrapper.findComponent(SortMenu).vm.$emit('update:modelValue', [{ property: 'due', direction: 'desc' }])
    await flushPromises()

    expect(lastParams()?.sort).toBe('-due')
    wrapper.unmount()
  })
})
