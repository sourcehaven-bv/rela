import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import KanbanView from './KanbanView.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, EntityWorld, ListResponse } from '@/types'

// A board is a projection through a world exactly as a list is, and until
// TKT-ILT1WD it was the one surface that said nothing about it: no `?world=`
// on its query, no per-card face provenance, and — worst of the three — cards
// that stayed draggable under a world whose writes the API refuses, so the
// gesture failed only AFTER the card had visibly moved.

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
  // Cards are real links since TKT-3CSZRG, so the component renders a
  // RouterLink; this mock replaces the whole module, so it must supply one.
  // `to` is exposed as data-to so a test can assert WHERE a card goes.
  RouterLink: {
    props: ['to'],
    template: '<a :data-to="JSON.stringify(to)"><slot /></a>',
  },
}))

vi.mock('@/composables/useBackTarget', () => ({
  useBackTarget: () => null,
}))

const KANBAN_ID = 'board'
const ENTITY_TYPE = 'procedure'

function seedSchema(opts: { editForm?: string } = {}) {
  const schemaStore = useSchemaStore()
  schemaStore.kanbans.set(KANBAN_ID, {
    entity: ENTITY_TYPE,
    title: 'Readiness',
    column_property: 'readiness',
    columns: [{ value: 'drilled', label: 'Drilled' }],
    card: { title: 'title' },
    create_form: 'new_procedure',
    ...(opts.editForm ? { edit_form: opts.editForm } : {}),
  } as never)
  schemaStore.entityTypes.set(ENTITY_TYPE, {
    name: ENTITY_TYPE,
    label: 'Procedure',
    properties: {
      title: { type: 'string' },
      readiness: { type: 'enum', values: ['drilled'] },
    },
    faces: { en: { label: 'English' }, nl: { label: 'Nederlands' } },
    bare_face: 'en',
  } as never)
  schemaStore.worlds.set('site-nl', { name: 'site-nl', readable: true } as never)
  return schemaStore
}

function card(id: string, title: string, world?: EntityWorld): Entity {
  return {
    id,
    type: ENTITY_TYPE,
    properties: { title, readiness: 'drilled' },
    relations: {},
    // The board's drag gate reads `_actions`; every card here is writable as
    // far as the ACL is concerned, so anything that stops the drag below is
    // the WORLD doing it and not a permission.
    _actions: { update: true, create: true },
    ...(world ? { _world: world } : {}),
  } as Entity
}

async function mountBoard(entities: Entity[], opts: { editForm?: string } = {}) {
  seedSchema(opts)
  const response: ListResponse<Entity> = {
    data: entities,
    meta: { total: entities.length, page: 1, per_page: 100, has_more: false },
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
  _setEntityPluralForTest(ENTITY_TYPE, 'procedures')
  listAllEntitiesMock.mockReset()
  mockRoute.query = {}
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('KanbanView world scoping', () => {
  it('sends the world to the API so the board is that world’s projection', async () => {
    mockRoute.query = { world: 'site-nl' }
    await mountBoard([card('PRC-1', 'Herstellen vanaf back-up')])
    expect(listAllEntitiesMock).toHaveBeenCalled()
    const params = listAllEntitiesMock.mock.calls[0][1]
    expect(params?.world).toBe('site-nl')
  })

  it('sends NO world param under the default world', async () => {
    // An empty `?world=` would be a different cache entry for the same board,
    // and the composable returns undefined precisely so it can be spread.
    await mountBoard([card('PRC-1', 'Restore from backup')])
    const params = listAllEntitiesMock.mock.calls[0][1]
    expect(params?.world).toBeUndefined()
  })
})

describe('KanbanView per-card world badge', () => {
  it('badges only the card that resolved to a stand-in, in the operator\'s words', async () => {
    mockRoute.query = { world: 'site-nl' }
    const store = seedSchema()
    store.worlds.set('site-nl', { name: 'site-nl', readable: true, messages: { stand_in: '{face}' } } as never)
    listAllEntitiesMock.mockResolvedValue({
      data: [
        card('PRC-1', 'Herstellen vanaf back-up', { name: 'site-nl', face: 'nl', via: 'chain', chain_position: 0 }),
        card('PRC-2', "Revoke a leaver's access", { name: 'site-nl', face: 'en', via: 'chain', chain_position: 1 }),
      ],
      meta: { total: 2, page: 1, per_page: 100, has_more: false },
      included: {},
      _actions: { create: true },
    } as never)
    const wrapper = mount(KanbanView, {
      props: { id: KANBAN_ID },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    // Anti-vacuity: both cards really rendered, so the single badge is a
    // statement about provenance and not about a board that came up short.
    expect(wrapper.text()).toContain('Herstellen vanaf back-up')
    expect(wrapper.text()).toContain("Revoke a leaver's access")

    const badges = wrapper.findAll('.world-badge')
    expect(badges).toHaveLength(1)
    // `{face}` is the served face's LABEL from the type's faces.
    expect(badges[0].text()).toBe('English')
    expect(badges[0].classes()).toContain('is-fallback')
  })

  it('renders no badge for a stand-in when the world declares no stand_in text', async () => {
    mockRoute.query = { world: 'site-nl' }
    const wrapper = await mountBoard([
      card('PRC-2', "Revoke a leaver's access", { name: 'site-nl', face: 'en', via: 'chain', chain_position: 1 }),
    ])
    expect(wrapper.text()).toContain("Revoke a leaver's access")
    expect(wrapper.find('.world-badge').exists()).toBe(false)
  })

  it('renders no badge under the default world', async () => {
    const wrapper = await mountBoard([card('PRC-1', 'Restore from backup')])
    expect(wrapper.text()).toContain('Restore from backup')
    expect(wrapper.find('.world-badge').exists()).toBe(false)
  })
})

// Under a world a card's drag and Edit go to the card's ADDRESS (`_self`,
// face included), so the affordances are `_actions` alone — the server
// computes them for the face each card shows. The world itself no longer
// withdraws anything.
describe('KanbanView write affordances under a world', () => {
  it('keeps cards draggable under a world when _actions permits it', async () => {
    mockRoute.query = { world: 'site-nl' }
    const wrapper = await mountBoard([card('PRC-1', 'Herstellen vanaf back-up')])
    expect(wrapper.find('.kanban-card').attributes('draggable')).toBe('true')
  })

  it('refuses the drag for a card whose served face is not writable', async () => {
    // The positive control above has `_actions.update: true`; this is the
    // same card with the server's verdict flipped, so the difference is the
    // verdict and nothing else.
    mockRoute.query = { world: 'site-nl' }
    const c = card('PRC-1', 'Herstellen vanaf back-up')
    c._actions = { update: false, create: true }
    const wrapper = await mountBoard([c])
    expect(wrapper.find('.kanban-card').attributes('draggable')).toBe('false')
  })

  it('keeps the create button under a world when _actions permits it', async () => {
    mockRoute.query = { world: 'site-nl' }
    const wrapper = await mountBoard([card('PRC-1', 'Herstellen vanaf back-up')])
    expect(wrapper.text()).toContain('+ New')
  })

  it('shows the create button in the default world', async () => {
    const wrapper = await mountBoard([card('PRC-1', 'Restore from backup')])
    expect(wrapper.text()).toContain('+ New')
  })

  // A card with `edit_form` opens the form on the card's ADDRESS, so an edit
  // from a world-bound board edits the face the card showed.
  it('sends a card to the edit form at its address under a world', async () => {
    mockRoute.query = { world: 'site-nl' }
    const c = card('PRC-1', 'Herstellen')
    c._self = '/api/v1/procedures/PRC-1@nl'
    const wrapper = await mountBoard([c], { editForm: 'edit_procedure' })
    const to = JSON.parse(wrapper.find('.kanban-card').attributes('data-to') ?? 'null')
    expect(to).toBe('/form/edit_procedure/PRC-1@nl')
  })

  it('sends the SAME card to the edit form in the default world', async () => {
    const wrapper = await mountBoard([card('PRC-1', 'Restore')], { editForm: 'edit_procedure' })
    const to = JSON.parse(wrapper.find('.kanban-card').attributes('data-to') ?? 'null')
    expect(to).toBe('/form/edit_procedure/PRC-1')
  })

  it('sends a card without an edit form to the detail page, carrying the world', async () => {
    mockRoute.query = { world: 'site-nl' }
    const wrapper = await mountBoard([card('PRC-1', 'Herstellen')])
    const to = JSON.parse(wrapper.find('.kanban-card').attributes('data-to') ?? 'null')
    expect(to).toEqual({ path: '/entity/procedure/PRC-1', query: { world: 'site-nl' } })
  })
})

describe('KanbanView world banner', () => {
  it('renders the operator\'s projection note on a board of a FACED type', async () => {
    // A board under a world is a projection: cards may be missing entirely.
    // What that says to a reader is the operator's `messages.projection`;
    // the app has no sentence of its own (TKT-5SZG2L).
    mockRoute.query = { world: 'site-nl' }
    const store = seedSchema()
    store.worlds.set('site-nl', {
      name: 'site-nl', readable: true, messages: { projection: 'Alleen vertaalde procedures.' },
    } as never)
    listAllEntitiesMock.mockResolvedValue({
      data: [card('PRC-1', 'Herstellen vanaf back-up')],
      meta: { total: 1, page: 1, per_page: 100, has_more: false },
      included: {},
      _actions: { create: true },
    } as never)
    const wrapper = mount(KanbanView, {
      props: { id: KANBAN_ID },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    const banner = wrapper.find('.world-banner')
    expect(banner.exists()).toBe(true)
    expect(banner.find('.world-banner__note').text()).toBe('Alleen vertaalde procedures.')
    expect(banner.text()).not.toContain('read-only')
  })

  it('renders the operator announcement when the world declares one', async () => {
    mockRoute.query = { world: 'site-nl' }
    const store = seedSchema()
    store.worlds.set('site-nl', {
      name: 'site-nl',
      readable: true,
      banner: 'Dutch site',
    } as never)
    listAllEntitiesMock.mockResolvedValue({
      data: [card('PRC-1', 'Herstellen vanaf back-up')],
      meta: { total: 1, page: 1, per_page: 100, has_more: false },
      included: {},
    } as never)
    const wrapper = mount(KanbanView, {
      props: { id: KANBAN_ID },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    expect(wrapper.find('.world-banner__label').text()).toBe('Dutch site')
  })

  it('renders no banner at all when the world declares neither banner nor note', async () => {
    // `site-nl` in the seed declares nothing, so nothing renders — the app
    // has no words of its own for a world.
    mockRoute.query = { world: 'site-nl' }
    const wrapper = await mountBoard([card('PRC-1', 'Herstellen vanaf back-up')])
    expect(wrapper.text()).toContain('Herstellen vanaf back-up')
    expect(wrapper.find('.world-banner').exists()).toBe(false)
  })

  it('renders no note on a board of a type WITHOUT faces', async () => {
    // A type without faces has one state, present in every world, so no card
    // can be missing on the world's account — the note would be false.
    mockRoute.query = { world: 'site-nl' }
    const store = seedSchema()
    store.entityTypes.set(ENTITY_TYPE, {
      name: ENTITY_TYPE,
      label: 'Procedure',
      properties: {
        title: { type: 'string' },
        readiness: { type: 'enum', values: ['drilled'] },
      },
    } as never)
    listAllEntitiesMock.mockResolvedValue({
      data: [card('PRC-1', 'Herstellen vanaf back-up')],
      meta: { total: 1, page: 1, per_page: 100, has_more: false },
      included: {},
      _actions: { create: true },
    } as never)
    const wrapper = mount(KanbanView, {
      props: { id: KANBAN_ID },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('Herstellen vanaf back-up')
    expect(wrapper.find('.world-banner').exists()).toBe(false)
  })

  it('renders no banner at all in the default world', async () => {
    const wrapper = await mountBoard([card('PRC-1', 'Restore from backup')])
    expect(wrapper.text()).toContain('Restore from backup')
    expect(wrapper.find('.world-banner').exists()).toBe(false)
  })
})
