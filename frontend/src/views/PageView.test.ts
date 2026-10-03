import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { defineComponent, h } from 'vue'
import PageView from './PageView.vue'
import { usePageStore } from '@/stores/pages'
import { useSchemaStore } from '@/stores/schema'
import { usePageHeaderOutlet, resetPageHeader } from '@/composables/usePageHeader'
import { ApiError } from '@/api/errors'
import type { SidebarPage } from '@/types'

// An entity page (`pages.<p>.entity_type`) shows one entity, the anchor. The
// page reads it for the header, and hands each tab the narrowing to it.

const getEntityMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  getEntity: (...args: unknown[]) => getEntityMock(...args),
}))

const replaceMock = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: replaceMock }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
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

const topicPage: SidebarPage = {
  label: 'Topic',
  entity_type: 'topic',
  badge: 'gezondheid',
  tabs: [
    { id: 'board', label: 'Board', view: 'kanban', target: 'taken_bord', scope: 'relation', relation: 'bestaat_uit', direction: 'outgoing' },
    { id: 'tijdlijn', label: 'Tijdlijn', view: 'gantt', target: 'portfolio', scope: 'root' },
  ],
}

let pinia: ReturnType<typeof createPinia>
const mounted: VueWrapper[] = []

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  resetPageHeader()
  getEntityMock.mockReset()
  replaceMock.mockReset()
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
