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
  it('badges only the card that resolved to a stand-in', async () => {
    mockRoute.query = { world: 'site-nl' }
    const wrapper = await mountBoard([
      card('PRC-1', 'Herstellen vanaf back-up', {
        name: 'site-nl',
        face: 'nl',
        via: 'chain',
        chain_position: 0,
      }),
      card('PRC-2', "Revoke a leaver's access", {
        name: 'site-nl',
        face: 'en',
        via: 'chain',
        chain_position: 1,
      }),
    ])
    // Anti-vacuity: both cards really rendered, so the single badge is a
    // statement about provenance and not about a board that came up short.
    expect(wrapper.text()).toContain('Herstellen vanaf back-up')
    expect(wrapper.text()).toContain("Revoke a leaver's access")

    const badges = wrapper.findAll('.world-badge')
    expect(badges).toHaveLength(1)
    expect(badges[0].text()).toBe('en')
    expect(badges[0].classes()).toContain('is-fallback')
  })

  it('renders no badge under the default world', async () => {
    const wrapper = await mountBoard([card('PRC-1', 'Restore from backup')])
    expect(wrapper.text()).toContain('Restore from backup')
    expect(wrapper.find('.world-badge').exists()).toBe(false)
  })
})

describe('KanbanView write affordances under a world', () => {
  it('makes cards undraggable under a world', async () => {
    // The bug this pins: the API refuses `?world=` on a write, so a drag that
    // the board accepts fails only after the card has visibly moved to another
    // column. Refusing the gesture is the honest ordering.
    mockRoute.query = { world: 'site-nl' }
    const wrapper = await mountBoard([card('PRC-1', 'Herstellen vanaf back-up')])
    expect(wrapper.find('.kanban-card').attributes('draggable')).toBe('false')
  })

  it('keeps the SAME card draggable in the default world', async () => {
    // The positive control: `_actions.update` is true in both tests, so the
    // difference above is the world and nothing else.
    const wrapper = await mountBoard([card('PRC-1', 'Restore from backup')])
    expect(wrapper.find('.kanban-card').attributes('draggable')).toBe('true')
  })

  it('withdraws the create button under a world', async () => {
    // Same reasoning as the drag: a create lands in the default world, so a
    // "+ New" on a world-bound board offers a write this request cannot carry.
    mockRoute.query = { world: 'site-nl' }
    const wrapper = await mountBoard([card('PRC-1', 'Herstellen vanaf back-up')])
    expect(wrapper.text()).not.toContain('+ New')
  })

  it('shows the create button in the default world', async () => {
    const wrapper = await mountBoard([card('PRC-1', 'Restore from backup')])
    expect(wrapper.text()).toContain('+ New')
  })

  // A card with `edit_form` opened the form from a world-bound board: a write
  // surface on the DEFAULT face of an entity shown at another. Under a world
  // the card goes to the detail page, carrying the world.
  it('sends a card to the detail page, not the edit form, under a world', async () => {
    mockRoute.query = { world: 'site-nl' }
    const wrapper = await mountBoard([card('PRC-1', 'Herstellen')], { editForm: 'edit_procedure' })
    const to = JSON.parse(wrapper.find('.kanban-card').attributes('data-to') ?? 'null')
    expect(to).toEqual({ path: '/entity/procedure/PRC-1', query: { world: 'site-nl' } })
  })

  it('sends the SAME card to the edit form in the default world', async () => {
    const wrapper = await mountBoard([card('PRC-1', 'Restore')], { editForm: 'edit_procedure' })
    const to = JSON.parse(wrapper.find('.kanban-card').attributes('data-to') ?? 'null')
    expect(to).toBe('/form/edit_procedure/PRC-1')
  })
})

describe('KanbanView world banner', () => {
  it('explains the read-only board and offers the way back', async () => {
    // Withdrawing the drag without saying why is a bug from the reader's
    // side: cards that will not move and no account of it.
    mockRoute.query = { world: 'site-nl' }
    const wrapper = await mountBoard([card('PRC-1', 'Herstellen vanaf back-up')])
    const banner = wrapper.find('.world-banner')
    expect(banner.exists()).toBe(true)
    expect(banner.text()).toContain('read-only')
    // Both halves the board withdraws: the write, and the projection.
    expect(banner.text()).toContain('cannot be dragged')
    expect(banner.text()).toContain('not on the board at all')
    // The exit. A world-bound board with no way out is the trap the detail
    // page's banner comment warns about.
    expect(banner.text()).toContain('Go to English')
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

  it('announces nothing when the world declares no banner', async () => {
    // `site-nl` in the seed has no `banner:`. The read-only note still
    // renders — it is not the operator's to suppress.
    mockRoute.query = { world: 'site-nl' }
    const wrapper = await mountBoard([card('PRC-1', 'Herstellen vanaf back-up')])
    expect(wrapper.find('.world-banner__label').exists()).toBe(false)
    expect(wrapper.find('.world-banner__note').exists()).toBe(true)
  })

  it('renders no banner at all in the default world', async () => {
    const wrapper = await mountBoard([card('PRC-1', 'Restore from backup')])
    expect(wrapper.text()).toContain('Restore from backup')
    expect(wrapper.find('.world-banner').exists()).toBe(false)
  })
})
