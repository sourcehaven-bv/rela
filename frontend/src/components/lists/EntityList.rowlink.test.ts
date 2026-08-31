import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

const routerPush = vi.fn()
const mockRoute = { query: {} as Record<string, string>, path: '/list/tickets-list', name: 'list' }
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush, replace: vi.fn() }),
  useRoute: () => mockRoute,
}))

// Rows navigate via a JavaScript click handler, which the browser cannot see.
// That means right-click offers no "Open Link in New Tab", and Cmd/Ctrl+click
// and middle-click do nothing — so the common "open three entities side by
// side to compare them" workflow is impossible (TKT-BB3TV0 / issue #1172).
//
// The fix is a real <a href> in the row. These tests assert the href EXISTS and
// POINTS somewhere correct — the browser behaviours themselves (modifier
// clicks, context menu) are the browser's, and are not simulable in jsdom.
// Asserting the href is asserting the thing we actually control.
const listId = 'tickets-list'
const entityType = 'ticket'

function makeEntity(id: string): Entity {
  return { id, type: entityType, properties: { title: `Ticket ${id}`, status: 'open' } } as Entity
}

function seedSchema() {
  const schemaStore = useSchemaStore()
  schemaStore.lists.set(listId, {
    id: listId,
    title: 'Tickets',
    entity: entityType,
    columns: [{ property: 'title', label: 'Title' }, { property: 'status', label: 'Status' }],
  } as never)
  schemaStore.entityTypes.set(entityType, {
    name: entityType,
    label: 'Ticket',
    properties: { title: { type: 'string', values: null }, status: { type: 'string', values: null } },
  } as never)
}

function seedEntities(entities: Entity[]) {
  const response: ListResponse<Entity> = {
    data: entities,
    meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
    included: {},
  }
  listEntitiesMock.mockResolvedValue(response)
}

let pinia: ReturnType<typeof createPinia>

function resetHarness() {
  pinia = createPinia()
  setActivePinia(pinia)
  _setEntityPluralForTest(entityType, 'tickets')
  listEntitiesMock.mockReset()
  routerPush.mockClear()
  mockRoute.query = {}
}

async function mountList() {
  seedSchema()
  const wrapper = mount(EntityList, {
    props: { listId },
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

describe('EntityList row links', () => {
  beforeEach(resetHarness)

  it('renders a real anchor in each row pointing at the entity', async () => {
    seedEntities([
      { id: 'TKT-1', type: entityType, properties: { title: 'One', status: 'open' } },
      { id: 'TKT-2', type: entityType, properties: { title: 'Two', status: 'done' } },
    ])
    const wrapper = await mountList()

    const links = wrapper.findAll('a.row-link')
    expect(links).toHaveLength(2)
    expect(links[0].attributes('href')).toContain('/entity/ticket/TKT-1')
    expect(links[1].attributes('href')).toContain('/entity/ticket/TKT-2')
  })

  it('carries exactly one anchor per row, not one per cell', async () => {
    // Two columns are configured. Wrapping every cell would put a link around
    // checkboxes and delete buttons, and would make the row read as several
    // links to a screen reader rather than one destination.
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'One', status: 'open' } }])
    const wrapper = await mountList()

    expect(wrapper.findAll('a.row-link')).toHaveLength(1)
  })

  it('preserves list state in the href so a new tab lands in the same context', async () => {
    // A row opened in a new tab should arrive with the same sort/filter/search
    // context a plain click would carry — otherwise prev/next and the back
    // button behave differently depending on HOW the row was opened, which is
    // the kind of inconsistency that makes people distrust the feature.
    mockRoute.query = { 'filter[status]': 'open', q: 'urgent' }
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'One', status: 'open' } }])
    const wrapper = await mountList()

    const href = wrapper.find('a.row-link').attributes('href') ?? ''
    expect(href).toContain('/entity/ticket/TKT-1')
    expect(href).toContain('filter%5Bstatus%5D=open')
    expect(href).toContain('q=urgent')
  })

  it('encodes a space as %20, the way router.push would', async () => {
    // The href and the click must put the SAME string in the address bar.
    // URLSearchParams serialises space as `+`; vue-router uses `%20`. Both
    // decode alike, so this never breaks navigation -- it just means the URL
    // differs depending on how the row was opened, which quietly undermines
    // the "same destination either way" promise the rest of these tests make.
    mockRoute.query = { q: 'two words' }
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'One', status: 'open' } }])
    const wrapper = await mountList()

    const href = wrapper.find('a.row-link').attributes('href') ?? ''
    expect(href).toContain('q=two%20words')
    expect(href).not.toContain('q=two+words')
  })

  it('percent-encodes a path that would otherwise split at a ? or #', async () => {
    // A raw href is parsed by the BROWSER, so an unencoded `?` in the path
    // becomes a query separator and everything after it leaves the path --
    // sending a Cmd+click somewhere a plain click never goes. router.push
    // treats the same value as a path and encodes it. Entity ids cannot
    // contain these characters, but a column's `link` config can.
    const listId2 = 'linked-list'
    const schemaStore = useSchemaStore()
    schemaStore.lists.set(listId2, {
      id: listId2,
      title: 'Tickets',
      entity: entityType,
      columns: [{ property: 'title', label: 'Title', link: 'document/doc?draft#v2' }],
    } as never)
    schemaStore.entityTypes.set(entityType, {
      name: entityType,
      label: 'Ticket',
      properties: { title: { type: 'string', values: null } },
    } as never)
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'One' } }])
    const wrapper = mount(EntityList, {
      props: { listId: listId2 },
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()

    const href = wrapper.find('a.row-link').attributes('href') ?? ''
    // The path segment survives intact instead of splitting into query+hash.
    expect(href).toContain('doc%3Fdraft%23v2')
    const beforeQuery = href.split('?')[0]
    expect(beforeQuery).toContain('TKT-1')
    wrapper.unmount()
  })

  it('still navigates when a NON-first cell is clicked', async () => {
    // Regression guard. Every cell body renders inside a wrapper element so the
    // markup is written once. If that shared wrapper carries the anchor's
    // `@click.stop`, the modifier applies to ALL columns — Vue resolves `.stop`
    // at compile time, so a ternary in the handler body cannot switch it off.
    // The non-first columns then swallow the click and the row's own handler
    // never runs.
    //
    // `display: contents` makes this nearly invisible by hand: the wrapper has
    // no box, so clicking the td's PADDING still reaches the row and navigates.
    // Only the rendered glyphs go dead — the exact pixels users aim at.
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'One', status: 'open' } }])
    const wrapper = await mountList()

    // Second configured column ('status'), not the linked first one.
    const cells = wrapper.findAll('tbody tr td')
    const statusCell = cells.find((c) => c.text().includes('open') && !c.text().includes('One'))
    expect(statusCell).toBeDefined()

    await statusCell!.find('.row-cell').trigger('click')
    expect(routerPush).toHaveBeenCalledTimes(1)
    expect(routerPush.mock.calls[0][0]).toMatchObject({ path: '/entity/ticket/TKT-1' })
  })

  it('still navigates in place on a plain left click', async () => {
    // The anchor must not REPLACE the existing behaviour: a plain click still
    // goes through the router (SPA navigation), not a full page load.
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'One', status: 'open' } }])
    const wrapper = await mountList()

    await wrapper.find('a.row-link').trigger('click')
    expect(routerPush).toHaveBeenCalledTimes(1)
    expect(routerPush.mock.calls[0][0]).toMatchObject({ path: '/entity/ticket/TKT-1' })
  })
})

// The mobile layout is a SEPARATE template from the desktop table, so it can
// regress independently — and did: the anchor landed on the table first and
// mobile cards stayed click-only until this test was added. Long-press
// "Open in New Tab" is the mobile counterpart of the desktop context menu.
describe('EntityList mobile card links', () => {
  let originalMatchMedia: typeof window.matchMedia

  beforeEach(() => {
    resetHarness()
    originalMatchMedia = window.matchMedia
    // isMobile is seeded from matchMedia at setup, so stub before mount.
    window.matchMedia = ((query: string) =>
      ({
        matches: query.includes('max-width: 768px'),
        media: query,
        addEventListener: () => {},
        removeEventListener: () => {},
      }) as unknown as MediaQueryList) as typeof window.matchMedia
  })

  afterEach(() => {
    window.matchMedia = originalMatchMedia
  })

  it('renders a real anchor on each mobile card title', async () => {
    const tickets = [makeEntity('T-1'), makeEntity('T-2')]
    seedEntities(tickets)
    const wrapper = await mountList()

    // Guard: prove we are on the mobile template, not the desktop table.
    expect(wrapper.find('.mobile-card-list').exists()).toBe(true)

    const links = wrapper.findAll('a.mobile-card-title')
    expect(links).toHaveLength(tickets.length)
    for (const [i, ticket] of tickets.entries()) {
      expect(links[i].attributes('href')).toContain(`/entity/${ticket.type}/${ticket.id}`)
    }
    wrapper.unmount()
  })

  it('still navigates in place on a plain tap', async () => {
    const ticket = makeEntity('T-1')
    seedEntities([ticket])
    const wrapper = await mountList()

    await wrapper.find('a.mobile-card-title').trigger('click')
    expect(routerPush).toHaveBeenCalledWith(
      expect.objectContaining({ path: `/entity/${ticket.type}/${ticket.id}` }),
    )
    wrapper.unmount()
  })
})
