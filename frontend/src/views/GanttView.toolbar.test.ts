import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { ref } from 'vue'
import GanttView from './GanttView.vue'
import SearchBox from '@/components/lists/SearchBox.vue'
import FilterBar from '@/components/lists/FilterBar.vue'
import SortMenu from '@/components/lists/SortMenu.vue'
import { withPageHeader } from '@/composables/pageHeaderTestHost'
import { useSchemaStore } from '@/stores/schema'
import type { GanttNode } from '@/api/gantts'

// A gantt has the list's search box and Filters menu, and a Sort menu for its
// rows. Search and filters are the server's: they ride on the gantt read, which
// applies them before the roll-up. The sort is the view's: it orders the
// siblings of what was read.

const getGanttMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  getGantt: (...args: unknown[]) => getGanttMock(...args),
  getEntity: vi.fn(),
  updateEntity: vi.fn(),
}))

const routeQuery = ref<Record<string, string>>({})
const routerReplace = vi.fn((to: { query?: Record<string, string> }) => {
  if (to?.query) routeQuery.value = { ...to.query }
  return Promise.resolve()
})
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: routerReplace }),
  useRoute: () => ({
    get query() {
      return routeQuery.value
    },
  }),
}))

function node(id: string, title: string, start: string, children: GanttNode[] = []): GanttNode {
  return { id, type: 'project', title, planned: { start, end: '2026-12-01' }, children }
}

const mounted: { unmount: () => void }[] = []

function mountGantt(query: Record<string, string> = {}, roots?: GanttNode[]) {
  setActivePinia(createPinia())
  const store = useSchemaStore()
  store.entityTypes.set('project', {
    label: 'Project',
    properties: { title: { type: 'string' }, status: { type: 'enum', values: ['open', 'done'] } },
  } as never)
  store.gantts.set('plan', {
    title: 'Plan',
    hierarchy: ['contains'],
    multi_parent: 'first',
    on_cycle: 'error',
    default_depth: 2,
    max_depth: 10,
    max_nodes: 2000,
    sources: { project: { start: 'planned_start', end: 'planned_end' } },
    filter_controls: [{ property: 'status' }],
  })
  routeQuery.value = query
  getGanttMock.mockResolvedValue({
    roots: roots ?? [node('A', 'Alpha', '2026-03-01'), node('B', 'Bravo', '2026-01-01')],
  })
  const wrapper = mount(withPageHeader(GanttView), {
    props: { id: 'plan' },
    global: { plugins: [getActivePinia()!, PiniaColada] },
    attachTo: document.body,
  })
  mounted.push(wrapper)
  return wrapper
}

const lastCall = () => getGanttMock.mock.calls[getGanttMock.mock.calls.length - 1]

beforeEach(() => {
  getGanttMock.mockReset()
  routerReplace.mockClear()
})

afterEach(() => {
  while (mounted.length) mounted.pop()!.unmount()
  document.body.innerHTML = ''
})

describe('GanttView toolbar', () => {
  it('reads without narrowing by default', async () => {
    mountGantt()
    await flushPromises()

    expect(lastCall()).toEqual(['plan', undefined])
  })

  it('sends the search from ?q= with the read', async () => {
    const wrapper = mountGantt({ q: 'alpha' })
    await flushPromises()

    expect(lastCall()).toEqual(['plan', undefined, { q: 'alpha' }])
    expect(wrapper.find<HTMLInputElement>('.search-box input').element.value).toBe('alpha')
  })

  it('sends a filter from the URL with the read', async () => {
    mountGantt({ 'filter[status]': 'done' })
    await flushPromises()

    expect(lastCall()).toEqual(['plan', undefined, { 'filter[status]': 'done' }])
  })

  it('does not send a filter on a property no source type has', async () => {
    mountGantt({ 'filter[ghost]': '1' })
    await flushPromises()

    expect(lastCall()).toEqual(['plan', undefined])
  })

  it('writes a typed search to the URL and reads again', async () => {
    const wrapper = mountGantt()
    await flushPromises()

    wrapper.findComponent(SearchBox).vm.$emit('update:modelValue', 'bravo')
    await flushPromises()

    expect(routerReplace).toHaveBeenCalledWith({ query: { q: 'bravo' } })
    expect(lastCall()).toEqual(['plan', undefined, { q: 'bravo' }])
  })

  it('reads again when a filter changes', async () => {
    const wrapper = mountGantt()
    await flushPromises()

    wrapper.findComponent(FilterBar).vm.$emit('filter', { status: { value: 'open' } })
    await flushPromises()

    expect(lastCall()).toEqual(['plan', undefined, { 'filter[status]': 'open' }])
  })

  it('says nothing matches when the narrowed chart is empty', async () => {
    const wrapper = mountGantt({ q: 'zzz' }, [])
    await flushPromises()

    expect(wrapper.text()).toContain('Nothing matches the search or filters.')
  })

  it('offers sorting by title, start and end', async () => {
    const wrapper = mountGantt()
    await flushPromises()

    const options = wrapper.findComponent(SortMenu).props('options') as { property: string }[]
    expect(options.map((o) => o.property)).toEqual(['title', 'start', 'end'])
  })

  it('orders the rows by the sort from ?sort= without reading again', async () => {
    const wrapper = mountGantt({ sort: 'start' })
    await flushPromises()

    const order = wrapper.findAll('.row').map((r) => r.attributes('data-node-id'))
    expect(order).toEqual(['B', 'A'])
    expect(getGanttMock).toHaveBeenCalledTimes(1)
  })

  it('writes the chosen sort to the URL and reorders the rows', async () => {
    const wrapper = mountGantt()
    await flushPromises()
    expect(wrapper.findAll('.row').map((r) => r.attributes('data-node-id'))).toEqual(['A', 'B'])

    wrapper.findComponent(SortMenu).vm.$emit('update:modelValue', [{ property: 'title', direction: 'desc' }])
    await flushPromises()

    expect(routerReplace).toHaveBeenCalledWith({ query: { sort: '-title' } })
    expect(wrapper.findAll('.row').map((r) => r.attributes('data-node-id'))).toEqual(['B', 'A'])
    expect(getGanttMock).toHaveBeenCalledTimes(1)
  })
})
