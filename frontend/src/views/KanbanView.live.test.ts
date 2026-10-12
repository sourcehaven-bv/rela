import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import { api } from '@/api/client'
import type { Entity, EntityLease, ListResponse } from '@/types'

// A claim changes no entity, so it sends no `entity:changed`; the board learns
// of it only through its live subscription (TKT-IMBF7U). The card then shows
// who holds the claim, in both card templates.

const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: vi.fn().mockResolvedValue(undefined),
}))

const subscriptions: { label: string; path: () => string | null; onChange: () => void }[] = []
vi.mock('@/composables/useLiveSubscription', async (orig) => ({
  ...(await orig<typeof import('@/composables/useLiveSubscription')>()),
  useLiveSubscription: (label: string, path: () => string | null, onChange: () => void) => {
    subscriptions.push({ label, path, onChange })
  },
}))

let routeQuery: Record<string, string> = {}
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: routeQuery, path: '/kanban/board' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))
vi.mock('@/composables/useBackTarget', () => ({ useBackTarget: () => null }))

const LEASE: EntityLease = {
  user: 'bob@example.com',
  tool: 'mcp',
  label: 'triage',
  since: '2026-10-12T10:00:00Z',
  status: '<img src=x onerror=alert(1)>',
}

function card(id: string, lease?: EntityLease): Entity {
  return {
    id,
    type: 'task',
    properties: { title: id, status: 'open', team: 'core' },
    relations: {},
    _actions: { update: true },
    ...(lease ? { _lease: lease } : {}),
  } as Entity
}

let pinia: ReturnType<typeof createPinia>

async function mountBoard(entities: Entity[], swimlanes = false) {
  const schema = useSchemaStore()
  schema.kanbans.set('board', {
    entity: 'task',
    title: 'Board',
    column_property: 'status',
    columns: [{ value: 'open', label: 'Open' }],
    ...(swimlanes
      ? { swimlane_property: 'team', swimlanes: [{ value: 'core', label: 'Core' }] }
      : {}),
    card: { title: 'title' },
  } as never)
  schema.entityTypes.set('task', {
    name: 'task',
    label: 'Task',
    properties: {
      title: { type: 'string' },
      status: { type: 'enum', values: ['open'] },
      team: { type: 'enum', values: ['core'] },
    },
  } as never)
  listAllEntitiesMock.mockResolvedValue({
    data: entities,
    meta: { total: entities.length, page: 1, per_page: 100, has_more: false },
    included: {},
    _actions: { create: true },
  } as ListResponse<Entity>)
  const w = mount(KanbanView, {
    props: { id: 'board' },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return w
}

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest('task', 'tasks')
  listAllEntitiesMock.mockReset()
  subscriptions.length = 0
  routeQuery = {}
})
afterEach(() => {
  document.body.innerHTML = ''
})

describe('KanbanView live subscription and claim badge', () => {
  it('subscribes the first 100 cards of the board query', async () => {
    await mountBoard([card('T-1')])
    const sub = subscriptions.find((s) => s.label === 'kanban-board')
    expect(sub?.path()).toBe('/api/v1/tasks?page=1&per_page=100')
  })

  it('subscribes the exact GET listEntities sends for page 1, with a world and a filter', async () => {
    routeQuery = { world: 'draft', 'filter[status]': 'open' }
    // Run the real paging loop down to the HTTP client and record what it sends.
    const sent: string[] = []
    const axios = api.getAxios()
    const adapter = axios.defaults.adapter
    axios.defaults.adapter = async (config) => {
      sent.push(axios.getUri({ baseURL: '/api/v1', url: config.url, params: config.params }))
      return {
        data: {
          data: [card('T-1')],
          meta: { total: 1, page: 1, per_page: 100, has_more: false },
          included: {},
          _actions: { create: true },
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }
    const real = await vi.importActual<typeof import('@/api')>('@/api')
    listAllEntitiesMock.mockImplementation(real.listAllEntities)
    try {
      const schema = useSchemaStore()
      schema.kanbans.set('board', {
        entity: 'task',
        title: 'Board',
        column_property: 'status',
        columns: [{ value: 'open', label: 'Open' }],
        card: { title: 'title' },
      } as never)
      schema.entityTypes.set('task', {
        name: 'task',
        label: 'Task',
        properties: { title: { type: 'string' }, status: { type: 'enum', values: ['open'] } },
      } as never)
      mount(KanbanView, {
        props: { id: 'board' },
        attachTo: document.body,
        global: { plugins: [pinia, PiniaColada] },
      })
      await flushPromises()
    } finally {
      axios.defaults.adapter = adapter
    }

    expect(sent).toHaveLength(1)
    expect(sent[0]).toContain('world=draft')
    expect(sent[0]).toContain('status')
    const sub = subscriptions.find((s) => s.label === 'kanban-board')
    expect(sub?.path()).toBe(sent[0])
  })

  it('refetches the board when the subscription reports a change', async () => {
    const w = await mountBoard([card('T-1')])
    expect(w.find('.lease-badge').exists()).toBe(false)

    listAllEntitiesMock.mockResolvedValue({
      data: [card('T-1', LEASE)],
      meta: { total: 1, page: 1, per_page: 100, has_more: false },
      included: {},
    })
    subscriptions.find((s) => s.label === 'kanban-board')!.onChange()
    await flushPromises()

    const badge = w.find('.kanban-card .lease-badge')
    expect(badge.find('.lease-badge__text').text()).toBe('Claimed by bob@example.com (triage)')
    expect(badge.attributes('title')).toContain('<img src=x onerror=alert(1)>')
    expect(w.find('.kanban-card img').exists()).toBe(false)
  })

  it('shows the badge on a swimlane card', async () => {
    const w = await mountBoard([card('T-1', LEASE)], true)
    expect(w.find('.kanban-card .lease-badge__text').text()).toBe(
      'Claimed by bob@example.com (triage)'
    )
  })
})
