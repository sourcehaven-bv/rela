import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada, useQueryCache } from '@pinia/colada'
import { ref } from 'vue'
import CalendarView from './CalendarView.vue'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import { _setEntityPluralForTest } from '@/api/entities'
import { entityKeys } from '@/queries/entities'
import type { Entity, ListResponse } from '@/types'

const listAllEntitiesMock = vi.fn()
const updateEntityMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
}))

const routerPush = vi.fn()
const routeQuery = ref<Record<string, string>>({})
// replace() must actually update the query, like a real router: the view keeps
// its view/date in the URL, so a mock that swallows the write would leave the
// component frozen on its initial period and quietly pass navigation tests.
const routerReplace = vi.fn((to: { query?: Record<string, string> }) => {
  if (to?.query) routeQuery.value = { ...to.query }
  return Promise.resolve()
})
vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: routerPush,
    replace: (to: { query?: Record<string, string> }) => routerReplace(to),
    get currentRoute() {
      return { value: { query: routeQuery.value, path: '/calendar/schedule' } }
    },
  }),
  useRoute: () => ({ query: routeQuery.value, path: '/calendar/schedule' }),
}))

const CAL_ID = 'schedule'

function listResponse(data: Entity[], hasMore = false): ListResponse<Entity> {
  return {
    data,
    meta: { page: 1, per_page: data.length, total: data.length, has_more: hasMore },
  } as ListResponse<Entity>
}

/** A task with an all-day `due` date. */
function task(id: string, due: string, extra: Record<string, unknown> = {}): Entity {
  return { id, type: 'task', properties: { title: `Task ${id}`, due, ...extra }, relations: {} }
}

/** A meeting with a timed `starts_at`. */
function meeting(id: string, startsAt: string, extra: Record<string, unknown> = {}): Entity {
  return {
    id,
    type: 'meeting',
    properties: { name: `Meeting ${id}`, starts_at: startsAt, ...extra },
    relations: {},
  }
}

interface SetupOptions {
  sources?: unknown[]
  responses?: Record<string, Entity[]>
  timezone?: string
  query?: Record<string, string>
  event?: unknown
  maxEventsPerDay?: number
  editForm?: string
  truncated?: boolean
}

// Every mounted view is torn down after its test. Without this each one stays
// alive reacting to the shared routeQuery ref, so a later test sees fetches
// from every component mounted before it — which looked exactly like a refetch
// loop in the component until the mounts were counted.
const mounted: { unmount: () => void }[] = []

function setup(opts: SetupOptions = {}) {
  const pinia = createPinia()
  setActivePinia(pinia)

  routeQuery.value = { date: '2026-08-22', ...(opts.query ?? {}) }

  const schemaStore = useSchemaStore()
  schemaStore.entityTypes.set('task', {
    label: 'Task',
    properties: {
      title: { type: 'string' },
      due: { type: 'date' },
      due_end: { type: 'date' },
      status: { type: 'enum', values: ['todo', 'done'] },
    },
  } as never)
  schemaStore.entityTypes.set('meeting', {
    label: 'Meeting',
    properties: {
      name: { type: 'string' },
      starts_at: { type: 'datetime' },
      room: { type: 'string' },
    },
  } as never)

  schemaStore.calendars.set(CAL_ID, {
    title: 'Schedule',
    default_view: 'month',
    week_start: 'monday',
    day_start: '08:00',
    day_end: '20:00',
    max_events_per_day: opts.maxEventsPerDay ?? 4,
    edit_form: opts.editForm,
    event: opts.event,
    sources: opts.sources ?? [
      { entity: 'task', date: 'due', summary: 'title', max_span: 31 },
    ],
  } as never)

  _setEntityPluralForTest('task', 'tasks')
  _setEntityPluralForTest('meeting', 'meetings')

  const uiStore = useUIStore()
  uiStore.setDatetimeTimezone(opts.timezone ?? 'UTC')

  listAllEntitiesMock.mockImplementation((type: string) =>
    Promise.resolve(listResponse(opts.responses?.[type] ?? [], opts.truncated ?? false))
  )

  const wrapper = mount(CalendarView, {
    props: { id: CAL_ID },
    global: { plugins: [pinia, PiniaColada] },
    attachTo: document.body,
  })
  mounted.push(wrapper)
  return wrapper
}

beforeEach(() => {
  listAllEntitiesMock.mockReset()
  updateEntityMock.mockReset()
  routerPush.mockReset()
  routerReplace.mockReset()
})

afterEach(() => {
  while (mounted.length) mounted.pop()!.unmount()
  document.body.innerHTML = ''
})

describe('CalendarView rendering', () => {
  it('renders a month grid with a weekday header and every day of the month', async () => {
    const wrapper = setup({ responses: { task: [] } })
    await flushPromises()

    expect(wrapper.findAll('.calendar-weekday')).toHaveLength(7)
    // August 2026 needs six rows starting Monday 27 July.
    const cells = wrapper.findAll('.calendar-day')
    expect(cells.length % 7).toBe(0)
    expect(cells.length).toBeGreaterThanOrEqual(28)
  })

  it('renders an all-day event on its date', async () => {
    const wrapper = setup({ responses: { task: [task('T-1', '2026-08-22')] } })
    await flushPromises()

    const chips = wrapper.findAll('.calendar-chip')
    expect(chips).toHaveLength(1)
    expect(chips[0].text()).toContain('Task T-1')
  })

  /**
   * The regression guard for the defect this ticket fixed in compareValues:
   * a datetime source used to render nothing at all, because the date window
   * bound could not be compared against an RFC3339 value.
   */
  it('renders events from a datetime source', async () => {
    const wrapper = setup({
      sources: [{ entity: 'meeting', date: 'starts_at', summary: 'name', max_span: 31 }],
      responses: { meeting: [meeting('M-1', '2026-08-22T14:30:00Z')] },
    })
    await flushPromises()

    const chips = wrapper.findAll('.calendar-chip')
    expect(chips).toHaveLength(1)
    expect(chips[0].text()).toContain('Meeting M-1')
    // A timed event shows its time; an all-day one does not.
    expect(chips[0].text()).toContain('14:30')
  })

  it('merges multiple sources of different types onto one grid', async () => {
    const wrapper = setup({
      sources: [
        { entity: 'task', date: 'due', summary: 'title', max_span: 31 },
        { entity: 'meeting', date: 'starts_at', summary: 'name', max_span: 31 },
      ],
      responses: {
        task: [task('T-1', '2026-08-22')],
        meeting: [meeting('M-1', '2026-08-22T09:00:00Z')],
      },
    })
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Task T-1')
    expect(text).toContain('Meeting M-1')
  })

  it('shows an empty message rather than a spinner when nothing is in range', async () => {
    const wrapper = setup({ responses: { task: [] } })
    await flushPromises()

    expect(wrapper.find('.calendar-empty').exists()).toBe(true)
    expect(wrapper.find('.loading-state').exists()).toBe(false)
  })

  it('warns when a source hit the page cap instead of looking merely quiet', async () => {
    const wrapper = setup({ responses: { task: [task('T-1', '2026-08-22')] }, truncated: true })
    await flushPromises()

    // The cap banner specifically: a shared class would also match the
    // long-span warning, so the test would pass on the wrong banner.
    expect(wrapper.find('.truncation-banner--cap').exists()).toBe(true)
    expect(wrapper.find('.truncation-banner--span').exists()).toBe(false)
  })
})

describe('CalendarView timezone handling', () => {
  /**
   * The day-assignment invariant: a timed event's cell is its date in the
   * DISPLAY timezone. The same instant lands on different days at the extremes
   * of the offset range, and the chip must follow the display zone rather than
   * the browser's.
   */
  it.each([
    ['Pacific/Kiritimati', '23'], // +14 → 23 Aug
    ['Pacific/Niue', '22'], // −11 → 22 Aug
  ])('places the same instant by display timezone (%s)', async (tz, expectedDay) => {
    const wrapper = setup({
      sources: [{ entity: 'meeting', date: 'starts_at', summary: 'name', max_span: 31 }],
      responses: { meeting: [meeting('M-1', '2026-08-22T12:00:00Z')] },
      timezone: tz,
    })
    await flushPromises()

    const cellWithChip = wrapper
      .findAll('.calendar-day')
      .find((c) => c.find('.calendar-chip').exists())
    expect(cellWithChip).toBeDefined()
    expect(cellWithChip!.find('.calendar-day-number').text()).toBe(expectedDay)
  })
})

describe('CalendarView navigation', () => {
  it('requests a window and moves it when navigating periods', async () => {
    const wrapper = setup({ responses: { task: [] } })
    await flushPromises()

    const firstParams = listAllEntitiesMock.mock.calls[0][1] as Record<string, string>
    expect(firstParams['filter[due][gte]']).toBe('2026-07-27T00:00:00.000Z')
    // Half-open: the bound is midnight AFTER the last visible day, so a late
    // event on that day is still inside the window.
    expect(firstParams['filter[due][lt]']).toBe('2026-09-07T00:00:00.000Z')

    await wrapper.findAll('.calendar-nav button')[2].trigger('click') // next
    await flushPromises()

    expect(routerReplace).toHaveBeenCalled()
    const calls = routerReplace.mock.calls
    const q = calls[calls.length - 1][0].query as Record<string, string>
    expect(q.date).toBe('2026-09-22')
  })

  it('switches to week view and requests a seven-day window', async () => {
    const wrapper = setup({ responses: { task: [] }, query: { view: 'week' } })
    await flushPromises()

    expect(wrapper.findAll('.calendar-day')).toHaveLength(7)
    const params = listAllEntitiesMock.mock.calls[0][1] as Record<string, string>
    expect(params['filter[due][gte]']).toBe('2026-08-17T00:00:00.000Z')
    expect(params['filter[due][lt]']).toBe('2026-08-24T00:00:00.000Z')
  })

  it('widens the lower bound only for a source that declares an end date', async () => {
    setup({
      sources: [{ entity: 'task', date: 'due', end_date: 'due_end', summary: 'title', max_span: 31 }],
      responses: { task: [] },
      query: { view: 'week' },
    })
    await flushPromises()

    const params = listAllEntitiesMock.mock.calls[0][1] as Record<string, string>
    // 31 days before Monday 17 August.
    expect(params['filter[due][gte]']).toBe('2026-07-17T00:00:00.000Z')
    expect(params['filter[due][lt]']).toBe('2026-08-24T00:00:00.000Z')
  })
})

describe('CalendarView overflow', () => {
  it('caps chips per day and expands in place on demand', async () => {
    const many = Array.from({ length: 7 }, (_, i) => task(`T-${i}`, '2026-08-22'))
    const wrapper = setup({ responses: { task: many }, maxEventsPerDay: 3 })
    await flushPromises()

    expect(wrapper.findAll('.calendar-chip')).toHaveLength(3)
    const more = wrapper.find('.calendar-more')
    expect(more.text()).toContain('+4 more')

    await more.trigger('click')
    expect(wrapper.findAll('.calendar-chip')).toHaveLength(7)
  })
})

describe('CalendarView chip fields', () => {
  it('renders configured fields and drops empty ones', async () => {
    const wrapper = setup({
      event: { fields: [{ property: 'status' }] },
      responses: {
        task: [task('T-1', '2026-08-22', { status: 'todo' }), task('T-2', '2026-08-23')],
      },
    })
    await flushPromises()

    const chips = wrapper.findAll('.calendar-chip')
    expect(chips[0].find('.calendar-chip-fields').exists()).toBe(true)
    // T-2 has no status: a dense surface renders nothing, not a placeholder.
    expect(chips[1].find('.calendar-chip-fields').exists()).toBe(false)
  })
})

/**
 * The cache-key contract, which had no test and so let a real bug through.
 *
 * An earlier version keyed the grid on `['entities','calendar',…]`. That reads
 * naturally but breaks two mechanisms silently: `useEvents` invalidates
 * `['entities',<type>]` on SSE events (which does not prefix-match a key whose
 * second element is the literal 'calendar'), and `beginOptimistic` writes
 * through `entityKeys.list(<type>)` (a different entry entirely, so a dragged
 * event snapped back until the refetch landed).
 *
 * Neither failure is visible in a normal render test — the grid still shows the
 * right events — which is exactly why this asserts on the key itself.
 */
describe('CalendarView cache keys', () => {
  it('writes fetched data under a key descending from entityKeys.list(type)', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    routeQuery.value = { date: '2026-08-22' }

    const schemaStore = useSchemaStore()
    schemaStore.entityTypes.set('task', {
      label: 'Task',
      properties: { title: { type: 'string' }, due: { type: 'date' } },
    } as never)
    schemaStore.calendars.set(CAL_ID, {
      title: 'Schedule',
      default_view: 'month',
      week_start: 'monday',
      day_start: '08:00',
      day_end: '20:00',
      max_events_per_day: 4,
      sources: [{ entity: 'task', date: 'due', summary: 'title', max_span: 31 }],
    } as never)
    _setEntityPluralForTest('task', 'tasks')
    useUIStore().setDatetimeTimezone('UTC')
    listAllEntitiesMock.mockImplementation(() =>
      Promise.resolve(listResponse([task('T-1', '2026-08-22')]))
    )

    const wrapper = mount(CalendarView, {
      props: { id: CAL_ID },
      global: { plugins: [pinia, PiniaColada] },
      attachTo: document.body,
    })
    await flushPromises()

    const cache = useQueryCache()
    // Every entry the grid populated must sit beneath ['entities','task',...],
    // the prefix both the SSE invalidation and the optimistic write target.
    // Entries under the prefix both the SSE invalidation and the optimistic
    // write target. Asserting on getEntries({key: list}) alone would be a
    // tautology over its own filter, so this also checks that NO entry the
    // component created sits outside that prefix, and that the window is
    // carried in a params segment beneath it.
    const scoped = cache.getEntries({ key: entityKeys.list('task') })
    expect(scoped.length).toBeGreaterThan(0)
    expect(scoped.some((e) => e.key.length > 3)).toBe(true)

    const all = cache.getEntries({})
    const entityEntries = all.filter((e) => e.key[0] === 'entities')
    expect(entityEntries.length).toBeGreaterThan(0)
    for (const entry of entityEntries) {
      expect(entry.key.slice(0, 3)).toEqual(['entities', 'task', 'list'])
    }

    wrapper.unmount()
  })
})

/**
 * Overlapping refetches must settle on the NEWEST window.
 *
 * Navigating quickly starts several fetches at once. Without a generation
 * guard, whichever resolves last wins — so clicking "next" twice can settle on
 * the first month's events while the period label reads the second. There is
 * no error and nothing in the UI admits the mismatch, which is what makes it
 * worth a test rather than a comment.
 */
describe('CalendarView refetch races', () => {
  it('ignores a slow earlier window that resolves after a newer one', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    routeQuery.value = { date: '2026-08-22' }

    const schemaStore = useSchemaStore()
    schemaStore.entityTypes.set('task', {
      label: 'Task',
      properties: { title: { type: 'string' }, due: { type: 'date' } },
    } as never)
    schemaStore.calendars.set(CAL_ID, {
      title: 'Schedule',
      default_view: 'month',
      week_start: 'monday',
      day_start: '08:00',
      day_end: '20:00',
      max_events_per_day: 4,
      sources: [{ entity: 'task', date: 'due', summary: 'title', max_span: 31 }],
    } as never)
    _setEntityPluralForTest('task', 'tasks')
    useUIStore().setDatetimeTimezone('UTC')

    // First call (August) hangs until we release it; the second (September)
    // resolves immediately. The August result therefore lands LAST.
    let releaseAugust!: (v: ListResponse<Entity>) => void
    const august = new Promise<ListResponse<Entity>>((resolve) => {
      releaseAugust = resolve
    })
    listAllEntitiesMock
      .mockImplementationOnce(() => august)
      .mockImplementation(() => Promise.resolve(listResponse([task('SEP-1', '2026-09-10')])))

    const wrapper = mount(CalendarView, {
      props: { id: CAL_ID },
      global: { plugins: [pinia, PiniaColada] },
      attachTo: document.body,
    })
    await flushPromises()

    // Navigate to September while August is still in flight.
    await wrapper.findAll('.calendar-nav button')[2].trigger('click')
    await flushPromises()

    // Now let the stale August response arrive.
    releaseAugust(listResponse([task('AUG-1', '2026-08-22')]))
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Task SEP-1')
    expect(text).not.toContain('Task AUG-1')

    wrapper.unmount()
  })
})

/**
 * Two sources over the SAME entity type is the documented way to express OR —
 * the filter language has none, so `where:` clauses within a source are ANDed
 * and a second source is the escape hatch.
 *
 * Any identity built from entity type (or type + date property) collides for
 * these, so one source renders the other's rows under the wrong colour. The
 * existing multi-source test uses two different types and passes either way,
 * which is exactly why this one exists.
 */
describe('CalendarView same-type sources', () => {
  it('keeps two sources over one entity type distinct', async () => {
    listAllEntitiesMock.mockClear()
    const wrapper = setup({
      sources: [
        { entity: 'task', date: 'due', summary: 'title', color: 'blue', max_span: 31, where: ['status = todo'] },
        { entity: 'task', date: 'due', summary: 'title', color: 'red', max_span: 31, where: ['status = done'] },
      ],
      responses: { task: [] },
    })
    await flushPromises()

    // One request per source, each carrying its own where clause.
    expect(listAllEntitiesMock).toHaveBeenCalledTimes(2)
    const params = listAllEntitiesMock.mock.calls.map((c) => c[1] as Record<string, string>)
    expect(params.map((p) => p['filter[status][eq]']).sort()).toEqual(['done', 'todo'])

    wrapper.unmount()
  })

  it('renders each same-type source with its own colour', async () => {
    // Both sources return one row; distinct colours prove they did not collide.
    listAllEntitiesMock
      .mockImplementationOnce(() => Promise.resolve(listResponse([task('T-todo', '2026-08-22')])))
      .mockImplementationOnce(() => Promise.resolve(listResponse([task('T-done', '2026-08-23')])))

    const wrapper = setup({
      sources: [
        { entity: 'task', date: 'due', summary: 'title', color: 'blue', max_span: 31 },
        { entity: 'task', date: 'due', summary: 'title', color: 'red', max_span: 31 },
      ],
      responses: { task: [] },
    })
    await flushPromises()

    expect(wrapper.find('.calendar-chip--blue').exists()).toBe(true)
    expect(wrapper.find('.calendar-chip--red').exists()).toBe(true)

    wrapper.unmount()
  })
})

/**
 * `where:` was validated at config load and then never applied — so a calendar
 * declaring `where: ["status != done"]` showed done tasks. An operator writing
 * a clause they believe scopes the grid is the failure this pins.
 */
describe('CalendarView where clauses', () => {
  it('pushes where clauses into the request alongside the date window', async () => {
    setup({
      sources: [
        {
          entity: 'task',
          date: 'due',
          summary: 'title',
          max_span: 31,
          where: ['status != done', 'priority = high'],
        },
      ],
      responses: { task: [] },
    })
    await flushPromises()

    const params = listAllEntitiesMock.mock.calls[0][1] as Record<string, string>
    expect(params['filter[status][ne]']).toBe('done')
    expect(params['filter[priority][eq]']).toBe('high')
    // The date window is still there.
    expect(params['filter[due][gte]']).toBeDefined()
    expect(params['filter[due][lt]']).toBeDefined()
  })

  it('maps comparison operators to their API names', async () => {
    setup({
      sources: [
        { entity: 'task', date: 'due', summary: 'title', max_span: 31, where: ['due >= 2026-01-01'] },
      ],
      responses: { task: [] },
    })
    await flushPromises()

    const params = listAllEntitiesMock.mock.calls[0][1] as Record<string, string>
    expect(params['filter[due][gte]']).toBe('2026-01-01')
  })
})
