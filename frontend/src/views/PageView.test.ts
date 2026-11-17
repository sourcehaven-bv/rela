import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { defineComponent, h, reactive } from 'vue'
import PageView from './PageView.vue'
import { usePageStore } from '@/stores/pages'
import { useSchemaStore } from '@/stores/schema'
import { usePageHeaderOutlet, resetPageHeader } from '@/composables/usePageHeader'
import { usePresentationOutlet, resetPresentation } from '@/composables/usePresentation'
import { useDetailPanel, resetDetailPanel } from '@/composables/useDetailPanel'
import PageSteps from '@/components/pages/PageSteps.vue'
import { ApiError } from '@/api/errors'
import type { SidebarPage } from '@/types'

// An entity page (`pages.<p>.entity_type`) shows one entity, the anchor. The
// page reads it for the header, and hands each tab the narrowing to it.

const getEntityMock = vi.fn()
const listEntitiesMock = vi.fn()
const getEntityPositionMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  getEntity: (...args: unknown[]) => getEntityMock(...args),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
  getEntityPosition: (...args: unknown[]) => getEntityPositionMock(...args),
}))
const getPageStatusMock = vi.fn()
vi.mock('@/api/pageStatus', () => ({
  getPageStatus: (...args: unknown[]) => getPageStatusMock(...args),
}))

const replaceMock = vi.fn()
const pushMock = vi.fn()
const routeState = reactive<{ path: string; query: Record<string, string> }>({ path: '/', query: {} })
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock, replace: replaceMock }),
  useRoute: () => routeState,
  RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
}))

/* Each view records the props the page passed it. */
function viewStub(name: string) {
  return {
    __esModule: true,
    default: defineComponent({
      name,
      inheritAttrs: false,
      setup: (_, { attrs }) => () => h('div', { class: `stub-${name}`, 'data-props': JSON.stringify(attrs) }),
    }),
  }
}
vi.mock('@/views/ListView.vue', () => viewStub('ListView'))
vi.mock('@/views/KanbanView.vue', () => viewStub('KanbanView'))
vi.mock('@/views/GanttView.vue', () => viewStub('GanttView'))
vi.mock('@/views/CalendarView.vue', () => viewStub('CalendarView'))
vi.mock('@/views/DocumentView.vue', () => viewStub('DocumentView'))
vi.mock('@/views/DashboardView.vue', () => viewStub('DashboardView'))
vi.mock('@/components/entity/EntityDetail.vue', () => viewStub('EntityDetail'))

const topicPage: SidebarPage = {
  label: 'Topic',
  entity_type: 'topic',
  badge: 'gezondheid',
  tabs: [
    { id: 'board', label: 'Board', view: 'kanban', target: 'taken_bord', scope: 'relation', relation: 'bestaat_uit', direction: 'outgoing' },
    { id: 'tijdlijn', label: 'Tijdlijn', view: 'gantt', target: 'portfolio', scope: 'root' },
    { id: 'rapport', label: 'Rapport', view: 'document', target: 'topic_rapport', scope: 'root' },
    { id: 'stand', label: 'Stand', view: 'dashboard', target: 'topic_stand', scope: 'root' },
  ],
}

let pinia: ReturnType<typeof createPinia>
const mounted: VueWrapper[] = []

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  resetPageHeader()
  getEntityMock.mockReset()
  listEntitiesMock.mockReset()
  getEntityPositionMock.mockReset()
  getPageStatusMock.mockReset()
  getPageStatusMock.mockResolvedValue({ steps: {} })
  replaceMock.mockReset()
  pushMock.mockReset()
  routeState.path = '/'
  routeState.query = {}
  resetPresentation()
  resetDetailPanel()
  usePageStore().set({ topic: topicPage })
  useSchemaStore().entityTypes.set('topic', {
    label: 'Topic',
    properties: {
      title: { type: 'string' },
      gezondheid: { type: 'enum', values: ['op_koers', 'risico'] },
    },
  } as never)
})

afterEach(() => {
  for (const w of mounted.splice(0)) w.unmount()
})

async function mountPage(props: { page: string; tab?: string; entity?: string }) {
  const wrapper = mount(PageView, { props, global: { plugins: [pinia, PiniaColada] } })
  mounted.push(wrapper)
  await flushPromises()
  return wrapper
}

function viewProps(wrapper: VueWrapper, name: string): Record<string, unknown> {
  return JSON.parse(wrapper.find(`.stub-${name}`).attributes('data-props') ?? '{}')
}

describe('PageView on an entity page', () => {
  it('opens the first tab of /p/<page>/<entity>', async () => {
    getEntityMock.mockResolvedValue({ id: 'TOP-1', type: 'topic', _title: 'Gezond werk', properties: {} })
    await mountPage({ page: 'topic', tab: 'TOP-1' })
    expect(replaceMock).toHaveBeenCalledWith('/p/topic/TOP-1/board')
  })

  it('titles the header with the anchor and draws its badge', async () => {
    getEntityMock.mockResolvedValue({
      id: 'TOP-1', type: 'topic', _title: 'Gezond werk', properties: { gezondheid: 'op_koers' },
    })
    await mountPage({ page: 'topic', entity: 'TOP-1', tab: 'board' })
    expect(getEntityMock).toHaveBeenCalledWith('topic', 'TOP-1')
    const outlet = usePageHeaderOutlet()
    expect(outlet.title.value).toBe('Gezond werk')
    const badge = mount(defineComponent({ render: () => outlet.badge.value?.() }), { global: { plugins: [pinia] } })
    expect(badge.text()).toContain('op_koers')
    badge.unmount()
  })

  it('hands a relation tab the page scope and a gantt tab the root', async () => {
    getEntityMock.mockResolvedValue({
      id: 'TOP-1', type: 'topic', properties: {}, _self: '/api/v1/topics/TOP-1@draft',
    })
    const board = await mountPage({ page: 'topic', entity: 'TOP-1', tab: 'board' })
    expect(viewProps(board, 'KanbanView')).toEqual({
      id: 'taken_bord', pageScope: { page: 'topic', tab: 'board', entity: 'TOP-1', ref: 'TOP-1@draft' },
    })
    const gantt = await mountPage({ page: 'topic', entity: 'TOP-1', tab: 'tijdlijn' })
    expect(viewProps(gantt, 'GanttView')).toEqual({ id: 'portfolio', root: 'TOP-1' })
  })

  it('hands a document tab the anchor as its entry and a dashboard tab the anchor to count for', async () => {
    getEntityMock.mockResolvedValue({ id: 'TOP-1', type: 'topic', properties: {} })
    const doc = await mountPage({ page: 'topic', entity: 'TOP-1', tab: 'rapport' })
    expect(viewProps(doc, 'DocumentView')).toEqual({ name: 'topic_rapport', entityId: 'TOP-1' })
    const dash = await mountPage({ page: 'topic', entity: 'TOP-1', tab: 'stand' })
    expect(viewProps(dash, 'DashboardView')).toEqual({ name: 'topic_stand', anchor: 'TOP-1' })
  })

  it('publishes the tab on screen for the Create menu, and clears it on leave', async () => {
    getEntityMock.mockResolvedValue({
      id: 'TOP-1', type: 'topic', properties: {}, _self: '/api/v1/topics/TOP-1@draft',
    })
    const wrapper = await mountPage({ page: 'topic', entity: 'TOP-1', tab: 'tijdlijn' })
    expect(usePageStore().current).toEqual({ page: 'topic', tab: 'tijdlijn', entity: 'TOP-1', ref: 'TOP-1@draft' })
    wrapper.unmount()
    expect(usePageStore().current).toBeUndefined()
  })

  it('marks the page as opening while the anchor loads', async () => {
    let resolve!: (e: unknown) => void
    getEntityMock.mockReturnValue(new Promise((r) => (resolve = r)))
    await mountPage({ page: 'topic', entity: 'TOP-1', tab: 'board' })
    expect(usePageStore().opening).toBe(true)
    expect(usePageStore().current).toBeUndefined()
    resolve({ id: 'TOP-1', type: 'topic', properties: {} })
    await flushPromises()
    expect(usePageStore().opening).toBe(false)
    expect(usePageStore().current).toMatchObject({ tab: 'board', entity: 'TOP-1' })
  })

  it('clears opening when it leaves before the anchor loads', async () => {
    getEntityMock.mockReturnValue(new Promise(() => {}))
    const wrapper = await mountPage({ page: 'topic', entity: 'TOP-1', tab: 'board' })
    expect(usePageStore().opening).toBe(true)
    wrapper.unmount()
    expect(usePageStore().opening).toBe(false)
  })

  it('opens again when it moves to another entity', async () => {
    getEntityMock.mockResolvedValueOnce({
      id: 'TOP-1', type: 'topic', properties: {}, _self: '/api/v1/topics/TOP-1@draft',
    })
    const wrapper = await mountPage({ page: 'topic', entity: 'TOP-1', tab: 'board' })
    expect(usePageStore().current).toMatchObject({ entity: 'TOP-1', ref: 'TOP-1@draft' })
    let resolve!: (e: unknown) => void
    getEntityMock.mockReturnValueOnce(new Promise((r) => (resolve = r)))
    await wrapper.setProps({ entity: 'TOP-2' })
    await flushPromises()
    expect(usePageStore().opening).toBe(true)
    expect(usePageStore().current).toBeUndefined()
    resolve({ id: 'TOP-2', type: 'topic', properties: {} })
    await flushPromises()
    expect(usePageStore().opening).toBe(false)
    expect(usePageStore().current).toEqual({ page: 'topic', tab: 'board', entity: 'TOP-2', ref: 'TOP-2' })
  })

  it('publishes no tab while the anchor is hidden or missing', async () => {
    getEntityMock.mockRejectedValue(new ApiError('Entity not found', { kind: 'http', status: 404, original: null }))
    await mountPage({ page: 'topic', entity: 'TOP-X', tab: 'board' })
    expect(usePageStore().current).toBeUndefined()
    expect(usePageStore().opening).toBe(false)
  })

  it('shows an empty state when the anchor is hidden or missing', async () => {
    getEntityMock.mockRejectedValue(new ApiError('Entity not found', { kind: 'http', status: 404, original: null }))
    const wrapper = await mountPage({ page: 'topic', entity: 'TOP-X', tab: 'board' })
    expect(wrapper.find('.stub-KanbanView').exists()).toBe(false)
    expect(wrapper.text()).toContain('There is no topic “TOP-X”')
    expect(usePageHeaderOutlet().frame.value).toBeNull()
  })

  it('does not redirect a page opened without an entity', async () => {
    const wrapper = await mountPage({ page: 'topic' })
    expect(replaceMock).not.toHaveBeenCalled()
    expect(getEntityMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Open a topic to see this page.')
  })
})

// A dashboard tab hands DashboardView the dashboard's name (TKT-5LQNLY); the
// default dashboard's tab has no target and passes nothing.
describe('PageView dashboard tabs', () => {
  beforeEach(() => {
    usePageStore().set({
      sales: {
        label: 'Sales',
        tabs: [
          { id: 'pipeline', label: 'Pipeline', view: 'dashboard', target: 'crm' },
          { id: 'home', label: 'Home', view: 'dashboard' },
        ],
      },
    })
  })

  it('passes a named dashboard tab its name', async () => {
    const wrapper = await mountPage({ page: 'sales', tab: 'pipeline' })
    expect(viewProps(wrapper, 'DashboardView')).toEqual({ name: 'crm' })
  })

  it('passes the default dashboard tab no name', async () => {
    const wrapper = await mountPage({ page: 'sales', tab: 'home' })
    expect(viewProps(wrapper, 'DashboardView')).toEqual({})
  })
})

const workshopPage: SidebarPage = {
  label: 'Workshop',
  mode: 'steps',
  tabs: [
    { id: 'premortem', label: 'Pre-mortem', view: 'list', target: 'risks', prompt: 'What could go wrong?' },
    { id: 'matrix', label: 'Matrix', view: 'kanban', target: 'matrix' },
    { id: 'measures', label: 'Measures', view: 'list', target: 'measures' },
  ],
}

describe('PageView on a steps page', () => {
  beforeEach(() => {
    usePageStore().set({ topic: topicPage, workshop: workshopPage })
  })

  it('draws numbered steps instead of a tab bar, the current one marked', async () => {
    const w = await mountPage({ page: 'workshop', tab: 'matrix' })
    expect(usePageHeaderOutlet().frame.value?.tabs).toEqual([])
    const links = w.findAll('.page-steps__step')
    expect(links.map((l) => l.text())).toEqual(['1Pre-mortem', '2Matrix', '3Measures'])
    expect(links.map((l) => l.attributes('aria-current'))).toEqual([undefined, 'step', undefined])
    expect(links[2].attributes('href')).toBe('/p/workshop/measures')
    expect(w.find('.stub-KanbanView').exists()).toBe(true)
  })

  it('shows the prompt of the current step only', async () => {
    const first = await mountPage({ page: 'workshop', tab: 'premortem' })
    expect(first.find('.page-steps__prompt').text()).toBe('What could go wrong?')
    const second = await mountPage({ page: 'workshop', tab: 'matrix' })
    expect(second.find('.page-steps__prompt').exists()).toBe(false)
  })

  it('moves with Back and Next, each disabled at its end', async () => {
    const first = await mountPage({ page: 'workshop', tab: 'premortem' })
    const steps = first.findComponent(PageSteps)
    const [back, next] = steps.findAll('.page-steps__actions button')
    expect(back.attributes('disabled')).toBeDefined()
    await next.trigger('click')
    expect(pushMock).toHaveBeenCalledWith('/p/workshop/matrix')

    const last = await mountPage({ page: 'workshop', tab: 'measures' })
    const [lastBack, lastNext] = last.findAll('.page-steps__actions button')
    expect(lastNext.attributes('disabled')).toBeDefined()
    await lastBack.trigger('click')
    expect(pushMock).toHaveBeenCalledWith('/p/workshop/matrix')
  })

  it('presents with ?present=1, keeps it from step to step, and follows the route out', async () => {
    const w = await mountPage({ page: 'workshop', tab: 'matrix' })
    const outlet = usePresentationOutlet()
    expect(outlet.presenting.value).toBe(false)

    routeState.query = { present: '1' }
    await flushPromises()
    expect(outlet.presenting.value).toBe(true)
    expect(w.findAll('.page-steps__step')[2].attributes('href')).toBe('/p/workshop/measures?present=1')

    // The browser's back button drops the query; the shell comes back.
    routeState.query = {}
    await flushPromises()
    expect(outlet.presenting.value).toBe(false)
  })

  it('enters and leaves present mode through the URL, keeping the rest of the query', async () => {
    routeState.path = '/p/workshop/matrix'
    routeState.query = { world: 'draft', sort: 'title' }
    const w = await mountPage({ page: 'workshop', tab: 'matrix' })
    const present = () => w.findAll('.page-steps__actions button').slice(-1)[0]
    await present().trigger('click')
    expect(pushMock).toHaveBeenLastCalledWith({
      path: '/p/workshop/matrix',
      query: { world: 'draft', sort: 'title', present: '1' },
    })

    routeState.query = { world: 'draft', sort: 'title', present: '1' }
    await flushPromises()
    await present().trigger('click')
    expect(pushMock).toHaveBeenLastCalledWith({
      path: '/p/workshop/matrix',
      query: { world: 'draft', sort: 'title' },
    })
  })

  it('keeps the world in step links and drops other query keys', async () => {
    routeState.query = { world: 'draft', sort: 'title', present: '1' }
    const w = await mountPage({ page: 'workshop', tab: 'matrix' })
    expect(w.findAll('.page-steps__step')[2].attributes('href')).toBe(
      '/p/workshop/measures?world=draft&present=1'
    )
  })

  it('opens the first step of the bare page, presented if asked', async () => {
    routeState.query = { present: '1' }
    await mountPage({ page: 'workshop' })
    expect(replaceMock).toHaveBeenCalledWith('/p/workshop/premortem?present=1')
  })

  it('clears present mode when the page unmounts', async () => {
    routeState.query = { present: '1' }
    const w = await mountPage({ page: 'workshop', tab: 'matrix' })
    expect(usePresentationOutlet().presenting.value).toBe(true)
    w.unmount()
    mounted.splice(mounted.indexOf(w), 1)
    expect(usePresentationOutlet().presenting.value).toBe(false)
  })

  it('does not present an entity page whose entity is missing', async () => {
    usePageStore().set({ topicSteps: { ...topicPage, mode: 'steps' } })
    getEntityMock.mockRejectedValue(new ApiError('Entity not found', { kind: 'http', status: 404, original: null }))
    routeState.query = { present: '1' }
    const w = await mountPage({ page: 'topicSteps', entity: 'TOP-X', tab: 'board' })
    expect(usePresentationOutlet().presenting.value).toBe(false)
    expect(w.find('.page-steps').exists()).toBe(false)
  })

  it('ignores ?present=1 on a page with tabs', async () => {
    usePageStore().set({ tabs: { ...workshopPage, mode: undefined } })
    routeState.query = { present: '1' }
    const w = await mountPage({ page: 'tabs', tab: 'matrix' })
    expect(usePresentationOutlet().presenting.value).toBe(false)
    expect(w.find('.page-steps').exists()).toBe(false)
    expect(usePageHeaderOutlet().frame.value?.tabs).toHaveLength(3)
  })

  describe('PageDown and PageUp', () => {
    function press(key: string) {
      const event = new KeyboardEvent('keydown', { key, cancelable: true })
      window.dispatchEvent(event)
      return event
    }

    it('step while presenting', async () => {
      routeState.query = { present: '1' }
      await mountPage({ page: 'workshop', tab: 'matrix' })
      expect(press('PageDown').defaultPrevented).toBe(true)
      expect(pushMock).toHaveBeenLastCalledWith('/p/workshop/measures?present=1')
      press('PageUp')
      expect(pushMock).toHaveBeenLastCalledWith('/p/workshop/premortem?present=1')
    })

    it('do nothing past the last step', async () => {
      routeState.query = { present: '1' }
      await mountPage({ page: 'workshop', tab: 'measures' })
      press('PageDown')
      expect(pushMock).not.toHaveBeenCalled()
    })

    it('scroll as usual when not presenting', async () => {
      await mountPage({ page: 'workshop', tab: 'matrix' })
      expect(press('PageDown').defaultPrevented).toBe(false)
      expect(pushMock).not.toHaveBeenCalled()
    })

    it('leave the open detail panel alone', async () => {
      routeState.query = { present: '1' }
      await mountPage({ page: 'workshop', tab: 'matrix' })
      useDetailPanel().show({ component: defineComponent({ render: () => null }), props: {}, mode: 'inline' })
      expect(press('PageDown').defaultPrevented).toBe(false)
      expect(pushMock).not.toHaveBeenCalled()
    })

    it('leave a focused text field alone', async () => {
      routeState.query = { present: '1' }
      await mountPage({ page: 'workshop', tab: 'matrix' })
      const input = document.createElement('input')
      document.body.appendChild(input)
      input.focus()
      try {
        expect(press('PageDown').defaultPrevented).toBe(false)
        expect(pushMock).not.toHaveBeenCalled()
      } finally {
        input.remove()
      }
    })
  })
})

// A list step with `walk: true` shows the list's rows one at a time
// (TKT-M07C97Y); a step with `done_when` shows a check (TKT-MWZF0UO).
const walkPage: SidebarPage = {
  label: 'Review',
  mode: 'steps',
  tabs: [
    { id: 'intro', label: 'Intro', view: 'dashboard' },
    { id: 'each', label: 'Each risk', view: 'list', target: 'risks', walk: true, done_check: true },
    { id: 'wrap', label: 'Wrap up', view: 'list', target: 'measures', done_check: true },
  ],
}

function pos(current: number, total: number, prev?: string, next?: string) {
  return {
    current,
    total,
    prev: prev ? { id: prev, type: 'risk' } : null,
    next: next ? { id: next, type: 'risk' } : null,
  }
}

describe('PageView walking a list step', () => {
  beforeEach(() => {
    usePageStore().set({ review: walkPage })
    useSchemaStore().lists.set('risks', {
      entity: 'risk',
      title: 'Risks',
      default_sort: [{ property: 'title', direction: 'asc' }],
      filters: [{ property: 'status', operator: 'eq', value: 'open' }],
    } as never)
  })

  it('opens the first row of the list, keeping present mode and the world', async () => {
    routeState.query = { present: '1', world: 'draft' }
    listEntitiesMock.mockResolvedValue({ data: [{ id: 'RSK-1', type: 'risk' }], meta: {} })
    await mountPage({ page: 'review', tab: 'each' })
    expect(listEntitiesMock).toHaveBeenCalledWith(
      'risk',
      expect.objectContaining({ list_id: 'risks', per_page: 1, sort: 'title', world: 'draft' })
    )
    expect(replaceMock).toHaveBeenLastCalledWith('/p/review/each?world=draft&present=1&item=RSK-1')
  })

  it('says so when the list has no rows', async () => {
    listEntitiesMock.mockResolvedValue({ data: [], meta: {} })
    const w = await mountPage({ page: 'review', tab: 'each' })
    expect(w.text()).toContain('Nothing to go through')
    expect(replaceMock).not.toHaveBeenCalled()
  })

  it('shows the row in ?item= with its position and neighbours', async () => {
    routeState.query = { item: 'RSK-2' }
    getEntityPositionMock.mockResolvedValue(pos(2, 3, 'RSK-1', 'RSK-3'))
    const w = await mountPage({ page: 'review', tab: 'each' })
    const [id, scope] = getEntityPositionMock.mock.calls[0]
    expect(id).toBe('RSK-2')
    expect(scope).toEqual({
      source: 'list',
      type: 'risk',
      list_id: 'risks',
      sort: 'title',
      filters: { 'filter[status][eq]': 'open' },
    })
    expect(w.find('[data-testid="walk-position"]').text()).toBe('2 of 3')
    expect(w.find('[data-testid="walk-prev"]').attributes('href')).toBe('/p/review/each?item=RSK-1')
    expect(w.find('[data-testid="walk-next"]').attributes('href')).toBe('/p/review/each?item=RSK-3')
    expect(viewProps(w, 'EntityDetail')).toMatchObject({
      'entity-type': 'risk',
      'entity-id': 'RSK-2',
    })
    expect(w.find('.stub-ListView').exists()).toBe(false)
  })

  it('says a row outside the list is not in it, and links the first row', async () => {
    routeState.query = { item: 'RSK-X' }
    getEntityPositionMock.mockRejectedValue(
      new ApiError('not found', { kind: 'http', status: 404, original: null })
    )
    const w = await mountPage({ page: 'review', tab: 'each' })
    expect(w.text()).toContain('Not in this list')
    expect(w.find('.stub-EntityDetail').exists()).toBe(false)
    expect(w.find('.page-walk a').attributes('href')).toBe('/p/review/each')
  })

  it('drops the row from step links', async () => {
    routeState.query = { item: 'RSK-2', present: '1' }
    getEntityPositionMock.mockResolvedValue(pos(2, 3, 'RSK-1', 'RSK-3'))
    const w = await mountPage({ page: 'review', tab: 'each' })
    expect(w.findAll('.page-steps__step')[2].attributes('href')).toBe('/p/review/wrap?present=1')
  })

  it('moves through the rows on PageDown and PageUp before the steps', async () => {
    routeState.query = { item: 'RSK-1', present: '1' }
    getEntityPositionMock.mockResolvedValue(pos(1, 2, undefined, 'RSK-2'))
    await mountPage({ page: 'review', tab: 'each' })
    const press = (key: string) =>
      window.dispatchEvent(new KeyboardEvent('keydown', { key, cancelable: true }))
    press('PageDown')
    expect(pushMock).toHaveBeenLastCalledWith('/p/review/each?present=1&item=RSK-2')
    // No previous row: PageUp goes to the previous step.
    press('PageUp')
    expect(pushMock).toHaveBeenLastCalledWith('/p/review/intro?present=1')
  })

  it('goes to the next step after the last row', async () => {
    routeState.query = { item: 'RSK-2', present: '1' }
    getEntityPositionMock.mockResolvedValue(pos(2, 2, 'RSK-1'))
    await mountPage({ page: 'review', tab: 'each' })
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'PageDown', cancelable: true }))
    expect(pushMock).toHaveBeenLastCalledWith('/p/review/wrap?present=1')
  })

  it('holds PageDown while the position loads, rather than skip to the next step', async () => {
    routeState.query = { item: 'RSK-1', present: '1' }
    getEntityPositionMock.mockReturnValue(new Promise(() => {}))
    const w = await mountPage({ page: 'review', tab: 'each' })
    const event = new KeyboardEvent('keydown', { key: 'PageDown', cancelable: true })
    window.dispatchEvent(event)
    expect(event.defaultPrevented).toBe(true)
    expect(pushMock).not.toHaveBeenCalled()
    // The detail waits until the list confirms the row is one of its own.
    expect(w.find('.stub-EntityDetail').exists()).toBe(false)
  })

  it('says so when the rows cannot be read', async () => {
    listEntitiesMock.mockRejectedValue(
      new ApiError('boom', { kind: 'http', status: 500, original: null })
    )
    const w = await mountPage({ page: 'review', tab: 'each' })
    expect(w.text()).toContain('Could not read the rows of this step')
  })

  it('leaves a list step without walk as a list', async () => {
    const w = await mountPage({ page: 'review', tab: 'wrap' })
    expect(w.find('.stub-ListView').exists()).toBe(true)
    expect(listEntitiesMock).not.toHaveBeenCalled()
  })
})

describe('PageView done marks', () => {
  beforeEach(() => {
    usePageStore().set({ review: walkPage, workshop: workshopPage })
    listEntitiesMock.mockResolvedValue({ data: [], meta: {} })
  })

  it('checks the number of a step whose rule holds, and keeps Next enabled', async () => {
    getPageStatusMock.mockResolvedValue({ steps: { each: { done: false }, wrap: { done: true } } })
    const w = await mountPage({ page: 'review', tab: 'intro' })
    expect(getPageStatusMock).toHaveBeenCalledWith('review', undefined, undefined)
    const links = w.findAll('.page-steps__step')
    expect(links.map((l) => l.attributes('data-done'))).toEqual([undefined, undefined, 'true'])
    expect(links[2].find('.rl-icon').exists()).toBe(true)
    expect(links[2].text()).toContain('(done)')
    expect(links[1].text()).toBe('2Each risk')
    const next = w.findAll('.page-steps__actions button')[1]
    expect(next.attributes('disabled')).toBeUndefined()
  })

  it('judges a world-bound page in its world', async () => {
    routeState.query = { world: 'draft' }
    await mountPage({ page: 'review', tab: 'intro' })
    expect(getPageStatusMock).toHaveBeenCalledWith('review', undefined, 'draft')
  })

  it('asks nothing for a page without done_when', async () => {
    await mountPage({ page: 'workshop', tab: 'matrix' })
    expect(getPageStatusMock).not.toHaveBeenCalled()
  })

  it('judges an entity page for its entity', async () => {
    usePageStore().set({
      topicSteps: {
        ...topicPage,
        mode: 'steps',
        tabs: [{ ...topicPage.tabs[0], done_check: true }],
      },
    })
    getEntityMock.mockResolvedValue({ id: 'TOP-1', type: 'topic', properties: {} })
    await mountPage({ page: 'topicSteps', entity: 'TOP-1', tab: 'board' })
    expect(getPageStatusMock).toHaveBeenCalledWith('topicSteps', 'TOP-1', undefined)
  })
})
