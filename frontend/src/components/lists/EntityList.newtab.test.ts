import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// TKT-3CSZRG: list rows are real links so cmd/ctrl/middle-click opens a new tab.
//
// The anchor lives in RlTable's `name` slot and the library stretches ONE
// overlay over the whole row, so a click anywhere on the row is the anchor's
// own activation. rela no longer intercepts row clicks at all: the modifier
// handling that used to be `shouldDeferToBrowser` is now the browser's
// default action on a real link, which is why the tests below assert the
// ANCHOR and its href rather than a click that does or does not route.

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

    const link = wrapper.find('.rl-table-row__primary a')
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

    const href = wrapper.find('.rl-table-row__primary a').attributes('href')

    // The keyboard path (j/k then Enter) still pushes programmatically, and it
    // must land where the anchor points. Both read `rowTargets`, so asserting
    // the href against that map pins the two to one source — which is the
    // regression this guards: building the href separately drops the scope.
    const target = (wrapper.vm as unknown as {
      rowTargets: Map<string, { path: string; query: Record<string, string> }>
    }).rowTargets.get('TKT-1')
    expect(target, 'row should resolve a target').toBeTruthy()

    const params = new URLSearchParams()
    for (const [k, v] of Object.entries(target!.query)) params.append(k, String(v))
    expect(href).toBe(`${target!.path}?${params.toString()}`)
  })

  it('includes the list scope in the href, not just the bare entity path', async () => {
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'First' } }])
    const wrapper = await mountList()

    const href = wrapper.find('.rl-table-row__primary a').attributes('href') ?? ''
    expect(href).toContain('/entity/ticket/TKT-1')
    expect(href).toContain(`from=${listId}`)
    // Encoding-agnostic: the test stub and vue-router disagree on whether `:`
    // is percent-encoded, and the fact under test is that the scope is CARRIED,
    // not how it is spelled. open-new-tab.spec.ts pins the real encoding.
    expect(href).toMatch(new RegExp(`scope=list(:|%3A)${listId}`))
  })

  it('navigates through the anchor rather than a click handler', async () => {
    // Previously rela pushed the route from a handler on the <tr>. The row now
    // carries a real RouterLink stretched over it, so navigation is the link's
    // own: nothing intercepts the click, which is what makes cmd-click,
    // middle-click and "copy link address" work without special cases.
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'First' } }])
    const wrapper = await mountList()

    const link = wrapper.find('.rl-table-row__primary a')
    expect(link.exists()).toBe(true)
    expect(link.attributes('href')).toBeTruthy()
  })

  it.each([
    ['meta (cmd)', { metaKey: true }],
    ['ctrl', { ctrlKey: true }],
    ['shift', { shiftKey: true }],
    ['alt', { altKey: true }],
    ['middle button', { button: 1 }],
  ])('leaves a %s click to the browser instead of routing in place', async (_n, init) => {
    // The original bug was a row handler that called router.push on EVERY
    // click, so a cmd-click routed in place instead of opening a tab. Nothing
    // intercepts row clicks now, so the guarantee holds structurally — but it
    // is asserted rather than assumed, because re-adding a row click handler
    // is exactly how it would regress.
    seedEntities([{ id: 'TKT-1', type: entityType, properties: { title: 'First' } }])
    const wrapper = await mountList()

    await wrapper.find('.rl-table-row').trigger('click', init)

    expect(routerPush).not.toHaveBeenCalled()
  })

  it('renders no anchor when the entity type is empty', async () => {
    // entityDetailHref returns '' for an empty type; rendering an anchor would
    // emit /entity//<id>, which 404s.
    seedEntities([{ id: 'TKT-1', type: '', properties: { title: 'First' } }])
    const wrapper = await mountList()

    expect(wrapper.find('.rl-table-row__primary a').exists()).toBe(false)
    // The title still renders as plain text; clicking it must do nothing
    // rather than fall back to a handler that pushes an invalid path.
    await wrapper.find('.rl-table-row').trigger('click')
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
      const href = wrapper.find('.rl-table-row__primary a').attributes('href') ?? ''
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

      const href = wrapper.find('.rl-table-row__primary a').attributes('href') ?? ''
      expect(href).toContain('/document/spec/TKT-1')
    })
  })
})
