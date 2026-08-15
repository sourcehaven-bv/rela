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

  // TKT-ERHWL0. The template asks for a breakdown/table twice per card
  // (`v-if="…length"` then `v-for`). Both derivations are O(N) or worse over
  // the card's whole result set, so they must be memoized rather than
  // recomputed per call site.
  describe('derived card data is memoized', () => {
    function entity(id: string, props: Record<string, unknown>): Entity {
      return { id, type: 'ticket', properties: props } as unknown as Entity
    }

    it('derives a breakdown once even though the template reads it twice', async () => {
      // A getter on `status` counts every read of the property. Two template
      // call sites over 3 entities is 3 reads when memoized, 6 when not.
      let reads = 0
      const rows = ['todo', 'todo', 'done'].map((status, i) =>
        entity(`TKT-${i}`, {
          get status() {
            reads++
            return status
          },
        })
      )
      searchEntitiesMock.mockResolvedValue({
        data: rows,
        meta: { total: rows.length, page: 1, per_page: 50, has_more: false },
      })
      seedDashboard([card({ display: 'breakdown', group_by: 'status' })])

      const wrapper = await mountView()

      expect(wrapper.text()).toContain('todo')
      expect(reads).toBe(rows.length)
    })

    it('sorts table rows once even though the template reads them twice', async () => {
      // Counting reads of `title`, not just asserting the order: the order was
      // already correct before this change, so a bare `toEqual` pins sorting
      // and says nothing about how often the rows were derived. Deriving twice
      // costs a second sort pass, which this count catches.
      let reads = 0
      const values = ['c', 'a', 'b']
      const rows = values.map((t, i) =>
        entity(`TKT-${i}`, {
          get title() {
            reads++
            return t
          },
        })
      )
      searchEntitiesMock.mockResolvedValue({
        data: rows,
        meta: { total: rows.length, page: 1, per_page: 50, has_more: false },
      })
      seedDashboard([
        card({
          display: 'table',
          columns: [{ property: 'title' }],
          sort: [{ property: 'title', direction: 'asc' }],
        }),
      ])

      const wrapper = await mountView()

      const cells = wrapper.findAll('tbody td').map((td) => td.text())
      expect(cells).toEqual(['a', 'b', 'c'])

      // Sorting reads each row's property, then rendering reads it once per
      // cell. Deriving a second time re-runs the sort — measured at 15 reads
      // against the pre-TKT-ERHWL0 code, versus 9 here.
      const derivedTwice = 15
      expect(reads).toBeLessThan(derivedTwice)
    })

    // The collision this keying must not have. `cardKey()` covers
    // [title, query, display] — correct for `cardData`, where two cards with
    // one query share a fetch, but NOT sufficient to key derived data:
    // `group_by` changes the breakdown without changing the key. Keying the
    // derivation by cardKey rendered the second card's breakdown on both tiles.
    it('gives two cards differing only in group_by their own breakdown', async () => {
      searchEntitiesMock.mockResolvedValue({
        data: [entity('TKT-1', { status: 'todo', priority: 'high' })],
        meta: { total: 1, page: 1, per_page: 50, has_more: false },
      })
      seedDashboard([
        card({ title: 'T', query: 'type:ticket', display: 'breakdown', group_by: 'status' }),
        card({ title: 'T', query: 'type:ticket', display: 'breakdown', group_by: 'priority' }),
      ])

      const tiles = (await mountView()).findAll('.dashboard-card')

      expect(tiles).toHaveLength(2)
      expect(tiles[0].text()).toContain('todo')
      expect(tiles[1].text()).toContain('high')
    })

    // A count card renders one number, so it must not pay to copy and sort a
    // result set nothing displays.
    it('does not derive table rows for a count card', async () => {
      let reads = 0
      const rows = ['c', 'a', 'b'].map((t, i) =>
        entity(`TKT-${i}`, {
          get title() {
            reads++
            return t
          },
        })
      )
      searchEntitiesMock.mockResolvedValue({
        data: rows,
        meta: { total: rows.length, page: 1, per_page: 50, has_more: false },
      })
      seedDashboard([card({ display: 'count', sort: [{ property: 'title', direction: 'asc' }] })])

      const wrapper = await mountView()

      expect(wrapper.text()).toContain('3')
      expect(reads).toBe(0)
    })

    // The memo must not outlive the data it was derived from. `cardData` is a
    // ref holding a Map mutated with `.set()`, so the computed has to track
    // the ref itself — the arriving search response must reach the template.
    //
    // Note this asserts the memo invalidates *as data lands*, not on a later
    // reload: `loadData` runs only from `onMounted`, so a card list swapped
    // after mount does not refetch (pre-existing, unrelated to TKT-ERHWL0).
    it('renders data that arrives after the initial render', async () => {
      let resolveSearch!: (r: ListResponse<Entity>) => void
      searchEntitiesMock.mockReturnValue(
        new Promise<ListResponse<Entity>>((res) => {
          resolveSearch = res
        })
      )
      seedDashboard([card({ display: 'breakdown', group_by: 'status' })])

      // Mount without awaiting the search: the breakdown is empty, and the
      // computed has already been evaluated once in that state.
      const wrapper = mount(DashboardView)
      expect(wrapper.text()).not.toContain('done')

      resolveSearch({
        data: [entity('TKT-2', { status: 'done' })],
        meta: { total: 1, page: 1, per_page: 50, has_more: false },
      })
      await flushPromises()

      expect(wrapper.text()).toContain('done')
    })
  })
})
