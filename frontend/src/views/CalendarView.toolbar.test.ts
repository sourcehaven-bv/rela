import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { ref } from 'vue'
import CalendarView from './CalendarView.vue'
import SearchBox from '@/components/lists/SearchBox.vue'
import FilterBar from '@/components/lists/FilterBar.vue'
import { withPageHeader } from '@/composables/pageHeaderTestHost'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import { _setEntityPluralForTest } from '@/api/entities'
import type { ListResponse, Entity } from '@/types'

// A calendar has the list's search box and Filters menu. Both ride on every
// source's read: `q` goes to each source, and a filter goes to the sources
// whose entity type has the property.

const listAllEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: vi.fn(),
  getCommands: () => Promise.resolve([]),
}))

const routeQuery = ref<Record<string, string>>({})
const routerReplace = vi.fn((to: { query?: Record<string, string> }) => {
  if (to?.query) routeQuery.value = { ...to.query }
  return Promise.resolve()
})
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRouter: () => ({
    push: vi.fn(),
    replace: (to: { query?: Record<string, string> }) => routerReplace(to),
    get currentRoute() {
      return { value: { query: routeQuery.value, path: '/calendar/schedule' } }
    },
  }),
  useRoute: () => ({ query: routeQuery.value, path: '/calendar/schedule' }),
}))

const CAL_ID = 'schedule'
const mounted: { unmount: () => void }[] = []

function setup(query: Record<string, string> = {}, controls: unknown[] = [{ property: 'status' }]) {
  const pinia = createPinia()
  setActivePinia(pinia)
  routeQuery.value = { date: '2026-08-22', ...query }

  const schemaStore = useSchemaStore()
  schemaStore.entityTypes.set('task', {
    label: 'Task',
    properties: { title: { type: 'string' }, due: { type: 'date' }, status: { type: 'enum', values: ['todo', 'done'] } },
  } as never)
  schemaStore.entityTypes.set('meeting', {
    label: 'Meeting',
    properties: { name: { type: 'string' }, starts_at: { type: 'datetime' } },
  } as never)
  schemaStore.calendars.set(CAL_ID, {
    title: 'Schedule',
    default_view: 'month',
    week_start: 'monday',
    day_start: '08:00',
    day_end: '20:00',
    max_events_per_day: 4,
    filter_controls: controls,
    sources: [
      { entity: 'task', date: 'due', summary: 'title', max_span: 31, where: ['title != hidden'] },
      { entity: 'meeting', date: 'starts_at', summary: 'name', max_span: 31 },
    ],
  } as never)
  _setEntityPluralForTest('task', 'tasks')
  _setEntityPluralForTest('meeting', 'meetings')
  useUIStore().setDatetimeTimezone('UTC')

  listAllEntitiesMock.mockImplementation(() =>
    Promise.resolve({
      data: [] as Entity[],
      included: {},
      meta: { page: 1, per_page: 0, total: 0, has_more: false },
    } as ListResponse<Entity>),
  )
  const wrapper = mount(withPageHeader(CalendarView), {
    props: { id: CAL_ID },
    global: { plugins: [pinia, PiniaColada] },
    attachTo: document.body,
  })
  mounted.push(wrapper)
  return wrapper
}

/** The params of the newest read of `type`. */
function lastParams(type: string): Record<string, unknown> {
  const calls = listAllEntitiesMock.mock.calls.filter((c) => c[0] === type)
  return calls[calls.length - 1][1] as Record<string, unknown>
}

beforeEach(() => {
  listAllEntitiesMock.mockReset()
  routerReplace.mockClear()
})

afterEach(() => {
  while (mounted.length) mounted.pop()!.unmount()
  document.body.innerHTML = ''
})

describe('CalendarView toolbar', () => {
  it('sends the search to every source', async () => {
    setup({ q: 'launch' })
    await flushPromises()

    expect(lastParams('task').q).toBe('launch')
    expect(lastParams('meeting').q).toBe('launch')
  })

  it('sends no q without a search', async () => {
    setup()
    await flushPromises()

    expect(lastParams('task').q).toBeUndefined()
  })

  it('writes a typed search to the URL and keeps the calendar position', async () => {
    const wrapper = setup()
    await flushPromises()

    wrapper.findComponent(SearchBox).vm.$emit('update:modelValue', 'launch')
    await flushPromises()

    expect(routerReplace).toHaveBeenCalledWith({ query: { date: '2026-08-22', q: 'launch' } })
  })

  it('sends a filter only to the sources whose type has the property', async () => {
    setup({ 'filter[status]': 'done' })
    await flushPromises()

    expect(lastParams('task')['filter[status]']).toBe('done')
    // A meeting has no status: the filter does not narrow it, and is not sent,
    // because a read refuses a property its type lacks.
    expect(lastParams('meeting')['filter[status]']).toBeUndefined()
  })

  it('keeps the date window and the where: clause next to a filter on the same property', async () => {
    setup({ 'filter[due][gte]': '1999-01-01', 'filter[title][ne]': 'x' })
    await flushPromises()

    // The window bound and the where: clause are the view's, not the
    // reader's: the reader can narrow a source but never widen it.
    expect(lastParams('task')['filter[due][gte]']).not.toBe('1999-01-01')
    expect(lastParams('task')['filter[title][ne]']).toBe('hidden')
  })

  it('refetches when a filter changes', async () => {
    const wrapper = setup()
    await flushPromises()
    const before = listAllEntitiesMock.mock.calls.length

    wrapper.findComponent(FilterBar).vm.$emit('filter', { status: { value: 'done' } })
    await flushPromises()

    expect(listAllEntitiesMock.mock.calls.length).toBeGreaterThan(before)
    expect(lastParams('task')['filter[status]']).toBe('done')
  })

  it('offers the filter controls of the config as property controls', async () => {
    const wrapper = setup()
    await flushPromises()

    expect(wrapper.findComponent(FilterBar).props('config')).toEqual({ filter_controls: [{ property: 'status' }] })
  })

  it('leaves out a relation control, which a calendar cannot resolve', async () => {
    const wrapper = setup({}, [{ relation: 'owned-by' }])
    await flushPromises()

    expect(wrapper.findComponent(FilterBar).exists()).toBe(false)
  })

  it('shows the search box but no Filters menu without filter controls', async () => {
    const wrapper = setup({}, [])
    await flushPromises()

    expect(wrapper.findComponent(SearchBox).exists()).toBe(true)
    expect(wrapper.findComponent(FilterBar).exists()).toBe(false)
  })
})
