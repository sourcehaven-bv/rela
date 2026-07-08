import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// KanbanView fetches its board through the api layer (useQuery over
// listAllEntities) and moves cards via updateEntity. Mock the api functions,
// mirroring EntityList.test.ts.
const listAllEntitiesMock = vi.fn()
const updateEntityMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
}))

const routerPush = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush }),
  useRoute: () => ({ query: {}, path: '/kanban/board' }),
}))

vi.mock('@/composables/useBackTarget', () => ({
  useBackTarget: () => null,
}))

const KANBAN_ID = 'board'
const ENTITY_TYPE = 'ticket'

function makeTicket(id: string): Entity {
  return {
    id,
    type: ENTITY_TYPE,
    properties: { title: `Ticket ${id}`, status: 'todo' },
    relations: {},
  }
}

// seedSchema installs a kanban config whose card renders the given fields,
// plus a `verantwoordelijk_voor` relation type with a declared inverse so the
// incoming-direction path resolves via getInverseName.
function seedSchema(fields: Array<Record<string, unknown>>) {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set(KANBAN_ID, {
    entity: ENTITY_TYPE,
    title: 'Board',
    column_property: 'status',
    columns: [{ value: 'todo', label: 'Todo' }],
    card: { title: 'title', fields },
  } as never)
  schemaStore.entityTypes.set(ENTITY_TYPE, {
    name: ENTITY_TYPE,
    label: 'Ticket',
    properties: {
      title: { type: 'string', values: null },
      status: { type: 'enum', values: ['todo'] },
    },
  } as never)
  // Relation with a declared inverse: incoming edges land under `handled_by`.
  schemaStore.relationTypes.set('verantwoordelijk_voor', {
    id: 'verantwoordelijk_voor',
    inverse: { id: 'handled_by' },
  } as never)
}

function seedBoard(entities: Entity[], included: Record<string, Entity> = {}): ListResponse<Entity> {
  const response: ListResponse<Entity> = {
    data: entities,
    meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
    included,
  }
  listAllEntitiesMock.mockResolvedValue(response)
  return response
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest(ENTITY_TYPE, 'tickets')
  _setEntityPluralForTest('person', 'people')
  listAllEntitiesMock.mockReset()
  updateEntityMock.mockReset().mockResolvedValue(undefined)
  routerPush.mockClear()
})

afterEach(() => {
  document.body.innerHTML = ''
})

async function mountBoard(fields: Array<Record<string, unknown>>, entities: Entity[], included: Record<string, Entity> = {}) {
  seedSchema(fields)
  seedBoard(entities, included)
  const wrapper = mount(KanbanView, {
    props: { id: KANBAN_ID },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

// NOTE (RR-UKS8BW): these are SPA-only unit tests. They prove that GIVEN the
// wire shape (a `relations` map with the expected key + an `included` map) the
// component resolves and renders the target — nothing more. The listAllEntities
// mock injects those maps directly; it is NOT the real endpoint.
//
// In particular the two INCOMING cases inject the inverse key
// (`relations: { handled_by: [...] }` / `{ blocks_inverse: [...] }`) that the
// real list endpoint does NOT emit on this branch — that key is populated
// server-side only once TKT-ODHV2D merges. So a green here is NOT end-to-end
// proof that incoming card fields work; it assumes the ODHV2D wire contract.
// The server side of that contract is pinned by the Go test
// `TestListEndpoint_IncomingEdge_InverseKey_ODHV2DContract` in
// internal/dataentry, which skips until ODHV2D lands.
describe('KanbanView card relation fields (assumes TKT-ODHV2D wire contract)', () => {
  it('renders an outgoing relation target title resolved from included', async () => {
    const ticket = makeTicket('T-1')
    // Outgoing edge keyed by the relation name itself.
    ticket.relations = { verantwoordelijk_voor: ['PER-9'] }
    const included = {
      'PER-9': { id: 'PER-9', type: 'person', _title: 'Alice', properties: {} } as Entity,
    }
    const wrapper = await mountBoard(
      [{ relation: 'verantwoordelijk_voor', direction: 'outgoing', label: 'Owner' }],
      [ticket],
      included
    )

    const card = wrapper.find('.kanban-card')
    expect(card.exists()).toBe(true)
    expect(card.text()).toContain('Owner')
    expect(card.text()).toContain('Alice')
    // Requested includes because a card field is a relation.
    expect(listAllEntitiesMock).toHaveBeenCalledWith(
      ENTITY_TYPE,
      { include: '*' },
      expect.any(AbortSignal)
    )
    wrapper.unmount()
  })

  it('renders an incoming relation target via the declared inverse key (ODHV2D contract)', async () => {
    const ticket = makeTicket('T-2')
    // ODHV2D CONTRACT: incoming edges are serialized under the relation's
    // inverse (handled_by). The real list endpoint only emits this key once
    // TKT-ODHV2D merges (see the Go contract test); here we inject it to test
    // the SPA's resolution given that shape.
    ticket.relations = { handled_by: ['PER-7'] }
    const included = {
      'PER-7': { id: 'PER-7', type: 'person', _title: 'Bob', properties: {} } as Entity,
    }
    const wrapper = await mountBoard(
      [{ relation: 'verantwoordelijk_voor', direction: 'incoming' }],
      [ticket],
      included
    )

    const card = wrapper.find('.kanban-card')
    expect(card.text()).toContain('Bob')
    wrapper.unmount()
  })

  it('falls back to <relation>_inverse when no inverse is declared (ODHV2D contract)', async () => {
    const schemaStore = useSchemaStore()
    const ticket = makeTicket('T-3')
    // ODHV2D CONTRACT: same as above — the `<relation>_inverse` key is a
    // server-side emission that only exists once TKT-ODHV2D merges; injected
    // here to exercise the SPA fallback key derivation.
    ticket.relations = { blocks_inverse: ['PER-5'] }
    const included = {
      'PER-5': { id: 'PER-5', type: 'person', _title: 'Carol', properties: {} } as Entity,
    }
    seedSchema([{ relation: 'blocks', direction: 'incoming' }])
    // `blocks` has no declared inverse.
    schemaStore.relationTypes.set('blocks', { id: 'blocks' } as never)
    seedBoard([ticket], included)
    const wrapper = mount(KanbanView, {
      props: { id: KANBAN_ID },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()

    expect(wrapper.find('.kanban-card').text()).toContain('Carol')
    wrapper.unmount()
  })

  it('falls back to the raw id when a relation target is not in included', async () => {
    const ticket = makeTicket('T-4')
    ticket.relations = { verantwoordelijk_voor: ['PER-404'] }
    const wrapper = await mountBoard(
      [{ relation: 'verantwoordelijk_voor', direction: 'outgoing' }],
      [ticket],
      {}
    )
    expect(wrapper.find('.kanban-card').text()).toContain('PER-404')
    wrapper.unmount()
  })
})

describe('KanbanView card property fields', () => {
  it('renders a plain property value and does not request includes', async () => {
    const ticket = makeTicket('T-5')
    ticket.properties = { title: 'Ticket T-5', status: 'todo', priority: 'high' }
    const wrapper = await mountBoard([{ property: 'priority', label: 'Priority' }], [ticket])

    const card = wrapper.find('.kanban-card')
    expect(card.text()).toContain('Priority')
    expect(card.text()).toContain('high')
    // No relation field → no include param (property-only boards unchanged).
    expect(listAllEntitiesMock).toHaveBeenCalledWith(
      ENTITY_TYPE,
      undefined,
      expect.any(AbortSignal)
    )
    wrapper.unmount()
  })

  it('renders an enum property as a Badge', async () => {
    const ticket = makeTicket('T-6')
    ticket.properties = { title: 'Ticket T-6', status: 'todo' }
    const wrapper = await mountBoard([{ property: 'status' }], [ticket])

    // Badge component renders the enum value inside the card.
    const card = wrapper.find('.kanban-card')
    expect(card.find('.badge').exists()).toBe(true)
    expect(card.text()).toContain('todo')
    wrapper.unmount()
  })
})
