import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import { api } from '@/api/client'
import type { Entity, ListResponse } from '@/types'

// BUG-5OAQUG regression: the board must render entities from EVERY page of
// the list endpoint, not just page 1. Unlike KanbanView.test.ts (which mocks
// the '@/api' module boundary), this file mocks the HTTP client underneath —
// mocking '@/api' cannot intercept listAllEntities' module-internal call to
// listEntities, so only this seam lets the real pagination loop run inside
// the mounted component (RR-5YVXMK).
vi.mock('@/api/client', () => ({
  api: { get: vi.fn() },
}))

const getMock = vi.mocked(api.get)

const KANBAN_ID = 'board'
const ENTITY_TYPE = 'ticket'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/kanban/board' }),
}))

vi.mock('@/composables/useBackTarget', () => ({
  useBackTarget: () => null,
}))

function makeTicket(id: string, status = 'todo'): Entity {
  return {
    id,
    type: ENTITY_TYPE,
    properties: { title: `Ticket ${id}`, status },
    relations: {},
  }
}

function page(data: Entity[], meta: Partial<ListResponse<Entity>['meta']>): ListResponse<Entity> {
  return {
    data,
    meta: { total: data.length, page: 1, per_page: 100, has_more: false, ...meta },
    _actions: { create: true },
  }
}

function seedSchema() {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set(KANBAN_ID, {
    entity: ENTITY_TYPE,
    title: 'Board',
    column_property: 'status',
    columns: [
      { value: 'todo', label: 'Todo' },
      { value: 'waiting', label: 'Waiting' },
    ],
    card: { title: 'title' },
  } as never)
  schemaStore.entityTypes.set(ENTITY_TYPE, {
    name: ENTITY_TYPE,
    label: 'Ticket',
    properties: {
      title: { type: 'string', values: null },
      status: { type: 'enum', values: ['todo', 'waiting'] },
    },
  } as never)
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest(ENTITY_TYPE, 'tickets')
  getMock.mockReset()
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountBoard() {
  seedSchema()
  const wrapper = mount(KanbanView, {
    props: { id: KANBAN_ID },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

describe('KanbanView pagination (BUG-5OAQUG)', () => {
  it('renders cards from every page, including one only present on page 2', async () => {
    // The reporter's shape: a 'waiting' entity sorted onto page 2 vanished
    // while page-1 'waiting' entities rendered, so the column looked
    // plausibly complete.
    getMock
      .mockResolvedValueOnce(
        page([makeTicket('T-1'), makeTicket('T-2', 'waiting')], { total: 3, page: 1, has_more: true })
      )
      .mockResolvedValueOnce(page([makeTicket('T-3', 'waiting')], { total: 3, page: 2 }))

    const wrapper = await mountBoard()

    const cardIds = wrapper.findAll('.kanban-card .card-id').map((n) => n.text())
    expect(cardIds).toEqual(['T-1', 'T-2', 'T-3'])

    // T-3 landed in its column, not just in the data set.
    const columns = wrapper.findAll('.kanban-column')
    expect(columns[1].text()).toContain('T-3')

    // Two page requests, second one asked for page 2.
    expect(getMock).toHaveBeenCalledTimes(2)
    expect(getMock.mock.calls[1][1]).toMatchObject({ page: 2 })

    // Complete fetch → no truncation banner.
    expect(wrapper.find('.truncation-banner').exists()).toBe(false)
    wrapper.unmount()
  })

  it('shows a truncation banner when the page cap is hit', async () => {
    let n = 0
    getMock.mockImplementation(async () =>
      page([makeTicket(`T-${++n}`)], { total: 9999, page: n, has_more: true })
    )

    const wrapper = await mountBoard()

    expect(getMock).toHaveBeenCalledTimes(50)
    const banner = wrapper.find('.truncation-banner')
    expect(banner.exists()).toBe(true)
    expect(banner.text()).toContain('Showing 50 of 9999 items')
    // The board still renders what it has.
    expect(wrapper.findAll('.kanban-card')).toHaveLength(50)
    wrapper.unmount()
  })
})
