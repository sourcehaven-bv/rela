import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// TKT-3CSZRG: list rows are real links so cmd/ctrl/middle-click opens a new tab.
// The row is a <tr> (which cannot be an anchor), so the link lives in the first
// cell and is stretched over the row by CSS.

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

const routerPush = vi.fn()
const mockRoute = { query: {} as Record<string, string>, path: '/list/tickets-list', name: 'list' }
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({ push: routerPush, replace: vi.fn() }),
  useRoute: () => mockRoute,
}))

const listId = 'tickets-list'
const entityType = 'ticket'

function seedSchema(columns: unknown[] = [{ property: 'title', label: 'Title' }]) {
  const schemaStore = useSchemaStore()
  schemaStore.lists.set(listId, {
    id: listId,
    title: 'Tickets',
    entity: entityType,
    columns,
  } as never)
  schemaStore.entityTypes.set(entityType, {
    name: entityType,
    label: 'Ticket',
    properties: { title: { type: 'string', values: null } },
  } as never)
}

function seedEntities(entities: Entity[]): ListResponse<Entity> {
  const response: ListResponse<Entity> = {
    data: entities,
    meta: { total: entities.length, page: 1, per_page: 25, has_more: false },
    included: {},
  }
  listEntitiesMock.mockResolvedValue(response)
  return response
}

let pinia: ReturnType<typeof createPinia>

async function mountList() {
  const wrapper = mount(EntityList, {
    props: { listId },
    attachTo: document.body,
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

describe('EntityList row links (new-tab affordance)', () => {
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest(entityType, 'tickets')
    listEntitiesMock.mockReset()
    routerPush.mockReset()
    mockRoute.query = {}
    seedSchema()
  })

  it('renders a real anchor in the row so right-click and middle-click work', async () => {
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'First' } }])
    const wrapper = await mountList()

    const link = wrapper.find('.entity-row .row-link')
    expect(link.exists()).toBe(true)
    expect(link.element.tagName).toBe('A')
    expect(link.attributes('href')).toBeTruthy()
  })

  it('the href carries the SAME query as the programmatic push', async () => {
    // The regression this guards: building the href separately from the push
    // silently drops the list scope, so a cmd-clicked tab lands on an unscoped
    // detail page (dead prev/next, wrong back target) while still looking fine.
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'First' } }])
    const wrapper = await mountList()

    const href = wrapper.find('.entity-row .row-link').attributes('href')

    await wrapper.find('.entity-row').trigger('click')
    expect(routerPush).toHaveBeenCalledTimes(1)
    const pushed = routerPush.mock.calls[0][0] as { path: string; query: Record<string, string> }

    const params = new URLSearchParams()
    for (const [k, v] of Object.entries(pushed.query)) params.append(k, String(v))
    expect(href).toBe(`${pushed.path}?${params.toString()}`)
  })

  it('includes the list scope in the href, not just the bare entity path', async () => {
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'First' } }])
    const wrapper = await mountList()

    const href = wrapper.find('.entity-row .row-link').attributes('href') ?? ''
    expect(href).toContain('/entity/ticket/TKT-1')
    expect(href).toContain(`from=${listId}`)
    // Encoding-agnostic: the test stub and vue-router disagree on whether `:`
    // is percent-encoded, and the fact under test is that the scope is CARRIED,
    // not how it is spelled. open-new-tab.spec.ts pins the real encoding.
    expect(href).toMatch(new RegExp(`scope=list(:|%3A)${listId}`))
  })

  it('a plain left-click still navigates in-SPA', async () => {
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'First' } }])
    const wrapper = await mountList()

    await wrapper.find('.entity-row').trigger('click')

    expect(routerPush).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['meta (cmd)', { metaKey: true }],
    ['ctrl', { ctrlKey: true }],
    ['shift', { shiftKey: true }],
    ['alt', { altKey: true }],
    ['middle button', { button: 1 }],
  ])('defers a %s click to the browser instead of routing in place', async (_n, init) => {
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'First' } }])
    const wrapper = await mountList()

    await wrapper.find('.entity-row').trigger('click', init)

    // No router.push: the browser acts on the row's own anchor, opening a tab
    // or window. Routing in place here is exactly the bug being fixed.
    expect(routerPush).not.toHaveBeenCalled()
  })

  it('renders no anchor when the entity type is empty', async () => {
    // entityDetailHref returns '' for an empty type; rendering an anchor would
    // emit /entity//<id>, which 404s.
    seedEntities([{ id: 'TKT-1', type: '', properties: { title: 'First' } }])
    const wrapper = await mountList()

    expect(wrapper.find('.entity-row .row-link').exists()).toBe(false)
    await wrapper.find('.entity-row').trigger('click')
    expect(routerPush).not.toHaveBeenCalled()
  })

  it('never renders an empty href', async () => {
    seedEntities([
      { id: 'TKT-1', type: entityType, properties: { title: 'First' } },
      { id: 'TKT-2', type: '', properties: { title: 'Second' } },
    ])
    const wrapper = await mountList()

    for (const a of wrapper.findAll('a')) {
      expect(a.attributes('href')).not.toBe('')
    }
  })

  // The `cellLink` path is the one server-supplied string that can reach an
  // href. Its resolver (EntityList.resolveLinkTarget, mirroring the Go copy in
  // internal/dataentry/views_handler.go) is a closed allowlist over `detail`
  // and `document/*`; anything else resolves to '' and must render no anchor.
  // Pinned on both sides because an href is a much sharper sink than the
  // router.push it replaced.
  describe('column link: values', () => {
    it.each([
      ['javascript:alert(1)'],
      ['JavaScript:alert(1)'],
      ['data:text/html,<script>alert(1)</script>'],
      ['//evil.example.com'],
      ['https://evil.example.com'],
    ])('never binds a hostile link: value as an href (%s)', async (link) => {
      seedSchema([{ property: 'title', label: 'Title', link }])
      seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'First' } }])
      const wrapper = await mountList()

      // resolveLinkTarget rejects it to '', so the row falls back to the safe
      // entity route rather than binding an attacker-chosen scheme.
      const href = wrapper.find('.entity-row .row-link').attributes('href') ?? ''
      expect(href).toContain('/entity/ticket/TKT-1')
      for (const a of wrapper.findAll('a')) {
        const value = a.attributes('href') ?? ''
        expect(value).not.toMatch(/^(javascript|data|vbscript):/i)
        expect(value).not.toMatch(/^\/\//)
        expect(value).not.toMatch(/^https?:/i)
      }
    })

    it('uses an allowlisted document link as the row target', async () => {
      seedSchema([{ property: 'title', label: 'Title', link: 'document/spec' }])
      seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'First' } }])
      const wrapper = await mountList()

      const href = wrapper.find('.entity-row .row-link').attributes('href') ?? ''
      expect(href).toContain('/document/spec/TKT-1')
    })
  })
})
