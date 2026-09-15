import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// A board's `query_scope:` decides which cards are MEMBERS of the board. The
// list endpoint is keyed by entity TYPE, so the server cannot tell which board
// is on screen — the client has to say. Without that, a board declaring
// `query_scope: archief` renders the type's default instead, which is the
// fail-open the whole feature exists to prevent: the config validates, an
// index is even derived for it, and the board silently shows the wrong rows.

const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: vi.fn().mockResolvedValue(undefined),
}))

const mockRoute: { query: Record<string, unknown>; path: string } = {
  query: {},
  path: '/kanban/board',
}
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => mockRoute,
  RouterLink: {
    props: ['to'],
    template: '<a :data-to="JSON.stringify(to)"><slot /></a>',
  },
}))

vi.mock('@/composables/useBackTarget', () => ({
  useBackTarget: () => null,
}))

const KANBAN_ID = 'board'
const ENTITY_TYPE = 'taak'

function seedSchema(opts: { queryScope?: string } = {}) {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set(KANBAN_ID, {
    entity: ENTITY_TYPE,
    title: 'Taken',
    column_property: 'status',
    columns: [{ value: 'todo', label: 'Te doen' }],
    card: { title: 'title' },
    ...(opts.queryScope ? { query_scope: opts.queryScope } : {}),
  } as never)
  schemaStore.entityTypes.set(ENTITY_TYPE, {
    name: ENTITY_TYPE,
    label: 'Taak',
    properties: {
      title: { type: 'string' },
      status: { type: 'enum', values: ['todo'] },
    },
  } as never)
  return schemaStore
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

async function mountBoard(opts: { queryScope?: string } = {}) {
  seedSchema(opts)
  const response: ListResponse<Entity> = {
    data: [card('TAAK-1', 'Open werk')],
    meta: { total: 1, page: 1, per_page: 100, has_more: false },
    included: {},
    _actions: { create: true },
  } as never
  listAllEntitiesMock.mockResolvedValue(response)
  const wrapper = mount(KanbanView, {
    props: { id: KANBAN_ID },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest(ENTITY_TYPE, 'taken')
  listAllEntitiesMock.mockReset()
  mockRoute.query = {}
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('KanbanView query scope', () => {
  it('sends the board’s configured query_scope', async () => {
    await mountBoard({ queryScope: 'archief' })
    expect(listAllEntitiesMock).toHaveBeenCalled()
    const params = listAllEntitiesMock.mock.calls[0][1]
    expect(params?.query_scope).toBe('archief')
  })

  it('sends NO query_scope param when the board declares none', async () => {
    // Omitted rather than empty: the server refuses a name that does not
    // resolve, and "" is such a name. Absent is what makes the entity type's
    // `default` apply.
    await mountBoard()
    const params = listAllEntitiesMock.mock.calls[0][1]
    expect(params?.query_scope).toBeUndefined()
  })

  it('sends `all` verbatim so a board can withdraw the type’s default', async () => {
    await mountBoard({ queryScope: 'all' })
    const params = listAllEntitiesMock.mock.calls[0][1]
    expect(params?.query_scope).toBe('all')
  })
})
