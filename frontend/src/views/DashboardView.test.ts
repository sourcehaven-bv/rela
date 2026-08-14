import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import DashboardView from './DashboardView.vue'
import { useSchemaStore } from '@/stores/schema'
import type { DashboardCard, AnalyzeResult, ListResponse, Entity } from '@/types'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/dashboard' }),
  RouterLink: { template: '<a><slot/></a>' },
}))

const searchEntitiesMock = vi.fn<() => Promise<ListResponse<Entity>>>()
const analyzeMock = vi.fn<() => Promise<AnalyzeResult>>()
vi.mock('@/api', () => ({
  searchEntities: () => searchEntitiesMock(),
  analyze: () => analyzeMock(),
}))

function card(overrides: Partial<DashboardCard> = {}): DashboardCard {
  return { title: 'Open', query: 'type:ticket', display: 'count', ...overrides }
}

/**
 * Seeds the store as already-loaded with the given cards, standing in for the
 * `/_dashboard` fetch the store performs on load.
 */
function seedDashboard(cards: DashboardCard[]) {
  const store = useSchemaStore()
  store.dashboard = { title: 'Overview', cards }
  store.loaded = true
  return store
}

async function mountView() {
  const wrapper = mount(DashboardView)
  await flushPromises()
  return wrapper
}

describe('DashboardView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    searchEntitiesMock.mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 50, has_more: false },
    })
    analyzeMock.mockResolvedValue({ errors: 0, warnings: 0, issues: [], byCheck: {} })
  })

  // The wiring assertion this file exists for (TKT-53KICM): the view must
  // render whatever the server returned, never a list it re-derived itself.
  it('renders the cards the server returned', async () => {
    seedDashboard([card({ title: 'Open' }), card({ title: 'Recent' })])

    const wrapper = await mountView()

    const titles = wrapper.findAll('.dashboard-card h3').map((h) => h.text())
    expect(titles).toEqual(['Open', 'Recent'])
  })

  // A card the server filtered out is simply absent from the payload; there is
  // nothing client-side to hide. This pins that the view has no second opinion
  // about visibility — no client-side ACL evaluation.
  it('renders no card the server omitted, even when it carries a permission', async () => {
    seedDashboard([card({ title: 'Open' })])

    const wrapper = await mountView()

    expect(wrapper.text()).not.toContain('Audit')
    expect(wrapper.findAll('.dashboard-card')).toHaveLength(1)
  })

  it('does not filter on card.permission client-side', async () => {
    // A permission-carrying card that reached the client MUST render: the
    // server already decided, and re-deciding here would double-gate.
    seedDashboard([card({ title: 'Audit', permission: 'admin:read' })])

    const wrapper = await mountView()

    expect(wrapper.text()).toContain('Audit')
  })

  it('shows the empty state when there are no cards', async () => {
    seedDashboard([])

    const wrapper = await mountView()

    expect(wrapper.find('.dashboard-empty').exists()).toBe(true)
    expect(wrapper.findAll('.dashboard-card')).toHaveLength(0)
  })

  // RR-TIO1XP: the empty state and the not-yet-loaded state look identical, so
  // the loading gate has to cover the store load. If it does not, the "no
  // cards" message flashes on every dashboard load.
  it('shows the loading state, not the empty state, while the store loads', async () => {
    const store = useSchemaStore()
    store.loaded = false
    let resolveLoad: () => void = () => {}
    const loadPromise = new Promise<void>((r) => {
      resolveLoad = r
    })
    store.load = vi.fn(() => loadPromise)

    const wrapper = mount(DashboardView)
    await flushPromises()

    expect(wrapper.find('.loading-state').exists()).toBe(true)
    expect(wrapper.find('.dashboard-empty').exists()).toBe(false)

    store.dashboard = { title: 'Overview', cards: [card({ title: 'Open' })] }
    store.loaded = true
    resolveLoad()
    await flushPromises()

    expect(wrapper.find('.loading-state').exists()).toBe(false)
    expect(wrapper.text()).toContain('Open')
  })

  // Card data is keyed by card identity, not array position. With an
  // index-keyed map, dropping the FIRST of two cards leaves the survivor
  // rendering the dropped card's number — a wrong figure presented as fact.
  // The per-principal card list makes a mid-life shape change plausible.
  it('never binds one card’s data to another card’s tile', async () => {
    const store = seedDashboard([card({ title: 'A', query: 'q-a' }), card({ title: 'B', query: 'q-b' })])
    searchEntitiesMock.mockImplementation(() =>
      Promise.resolve({
        data: [],
        meta: { total: searchEntitiesMock.mock.calls.length === 1 ? 100 : 200, page: 1, per_page: 50, has_more: false },
      }),
    )

    const wrapper = await mountView()
    expect(wrapper.text()).toContain('100')
    expect(wrapper.text()).toContain('200')

    // A now disappears (e.g. its permission was revoked); B must keep its own
    // count of 200 and must not inherit A's 100.
    store.dashboard = { title: 'Overview', cards: [card({ title: 'B', query: 'q-b' })] }
    await flushPromises()

    const tiles = wrapper.findAll('.dashboard-card')
    expect(tiles).toHaveLength(1)
    expect(tiles[0].text()).toContain('B')
    expect(tiles[0].text()).toContain('200')
    expect(tiles[0].text()).not.toContain('100')
  })

  it('queries the search endpoint once per rendered card', async () => {
    seedDashboard([card({ title: 'Open' }), card({ title: 'Recent' })])

    await mountView()

    expect(searchEntitiesMock).toHaveBeenCalledTimes(2)
  })
})
