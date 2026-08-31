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
// fetchView is what EntityDetail loads its content from. Without it stubbed the
// component mounts but renders no header at all, which would make the
// "no toolbar in the preview" assertion below vacuously true.
const fetchViewMock = vi.fn(() =>
  Promise.resolve({
    entry: {
      id: 'T-1',
      type: 'task',
      properties: { title: 'Task T-1', due: '2026-08-22' },
      relations: {},
    },
    sections: [],
  })
)
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
  fetchView: (...args: unknown[]) => fetchViewMock(...(args as [])),
  getCommands: () => Promise.resolve([]),
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
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
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

function listResponse(
  data: Entity[],
  hasMore = false,
  included: Record<string, unknown> = {}
): ListResponse<Entity> {
  return {
    data,
    included,
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
  /** Entities the server embedded via ?include=*, keyed by id. */
  included?: Record<string, unknown>
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
    Promise.resolve(
      listResponse(opts.responses?.[type] ?? [], opts.truncated ?? false, opts.included ?? {})
    )
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
    // `.card-fields` comes from the shared CardFieldList, which kanban cards
    // render too — the two surfaces now use one implementation.
    expect(chips[0].find('.card-fields').exists()).toBe(true)
    // T-2 has no status: a dense surface renders nothing, not a placeholder.
    expect(chips[1].find('.card-fields').exists()).toBe(false)
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

/**
 * Clicking a chip PREVIEWS rather than edits.
 *
 * The previous behaviour navigated straight into `edit_form` when one was
 * configured, so a click on a small chip — the natural "what is this?" gesture
 * — landed the user in a form with a save button. The modal answers that
 * question first; editing is a deliberate second step from inside it.
 */
describe('CalendarView event click', () => {
  it('opens a preview modal instead of navigating', async () => {
    const wrapper = setup({
      responses: { task: [task('T-1', '2026-08-22')] },
      editForm: 'edit_task',
    })
    await flushPromises()

    await wrapper.find('.calendar-chip').trigger('click')
    await flushPromises()

    // No navigation: the user stays on the calendar, keeping their period.
    expect(routerPush).not.toHaveBeenCalled()
    expect(document.querySelector('.entity-preview')).not.toBeNull()
  })

  it('shows no edit, history or delete toolbar in the preview', async () => {
    // A preview is a read surface. EntityDetail brings its own toolbar, which
    // would put a destructive Delete one click from a calendar chip and a
    // second Edit next to the footer's. It is rendered with `hide-actions`;
    // this asserts that stays true.
    const wrapper = setup({
      responses: { task: [task('T-1', '2026-08-22')] },
      editForm: 'edit_task',
    })
    await flushPromises()

    await wrapper.find('.calendar-chip').trigger('click')
    await flushPromises()

    const modal = document.querySelector('.entity-preview') as HTMLElement
    expect(modal).not.toBeNull()
    expect(modal.querySelector('.header-actions')).toBeNull()

    // The footer still offers exactly one Edit. Both actions are real links
    // now (TKT-3CSZRG) so cmd/middle-click opens them in a tab.
    const footerButtons = [...modal.querySelectorAll('.modal-actions button, .modal-actions a')].map((b) =>
      b.textContent?.trim()
    )
    expect(footerButtons).toEqual(['Open full page', 'Edit'])
  })

  it('omits Edit when the calendar configures no edit form', async () => {
    const wrapper = setup({ responses: { task: [task('T-1', '2026-08-22')] } })
    await flushPromises()

    await wrapper.find('.calendar-chip').trigger('click')
    await flushPromises()

    const modal = document.querySelector('.entity-preview') as HTMLElement
    const footerButtons = [...modal.querySelectorAll('.modal-actions button, .modal-actions a')].map((b) =>
      b.textContent?.trim()
    )
    expect(footerButtons).toEqual(['Open full page'])
  })

  it('closes the preview without navigating away', async () => {
    const wrapper = setup({ responses: { task: [task('T-1', '2026-08-22')] } })
    await flushPromises()

    await wrapper.find('.calendar-chip').trigger('click')
    await flushPromises()
    expect(document.querySelector('.entity-preview')).not.toBeNull()

    const close = document.querySelector('.entity-preview-close') as HTMLElement
    close.click()
    await flushPromises()

    expect(document.querySelector('.entity-preview')).toBeNull()
    expect(routerPush).not.toHaveBeenCalled()
  })
})

/**
 * Chip fields are the answer to "a calendar of bare titles doesn't tell me
 * much". A field may name a property or a RELATION, whose targets resolve to
 * their display titles — the relation half was declared in the config type but
 * silently skipped until it was implemented, so it is pinned here.
 */
describe('CalendarView chip detail', () => {
  it('renders property and relation fields together', async () => {
    const wrapper = setup({
      event: {
        fields: [{ property: 'assignee' }, { property: 'status' }, { relation: 'belongs-to' }],
      },
      responses: {
        task: [
          {
            id: 'T-1',
            type: 'task',
            properties: { title: 'Task T-1', due: '2026-08-22', assignee: 'Alex', status: 'todo' },
            relations: { 'belongs-to': ['PRJ-1'] },
          } as never,
        ],
      },
      // `_title` is the server-computed display title entityDisplayTitle reads;
      // it is not the `title` property (a type's display property may be `name`).
      included: { 'PRJ-1': { id: 'PRJ-1', type: 'project', _title: 'Apollo', properties: {} } },
    })
    await flushPromises()

    const chip = wrapper.find('.calendar-chip')
    expect(chip.text()).toContain('Task T-1')
    expect(chip.text()).toContain('Alex')
    // The relation resolves to the target's display title, not its raw id.
    expect(chip.text()).toContain('Apollo')
    expect(chip.text()).not.toContain('PRJ-1')
  })

  it('asks the server to embed related entities only when a relation field is configured', async () => {
    listAllEntitiesMock.mockClear()
    setup({ event: { fields: [{ property: 'status' }] }, responses: { task: [] } })
    await flushPromises()
    expect((listAllEntitiesMock.mock.calls[0][1] as Record<string, string>).include).toBeUndefined()

    listAllEntitiesMock.mockClear()
    setup({ event: { fields: [{ relation: 'belongs-to' }] }, responses: { task: [] } })
    await flushPromises()
    expect((listAllEntitiesMock.mock.calls[0][1] as Record<string, string>).include).toBe('*')
  })

  it('falls back to the raw id when a relation target was not embedded', async () => {
    const wrapper = setup({
      event: { fields: [{ relation: 'belongs-to' }] },
      responses: {
        task: [
          {
            id: 'T-1',
            type: 'task',
            properties: { title: 'Task T-1', due: '2026-08-22' },
            relations: { 'belongs-to': ['PRJ-9'] },
          } as never,
        ],
      },
    })
    await flushPromises()

    // Degrades visibly rather than rendering a blank field. This leaks nothing:
    // the server strips ACL-hidden neighbour ids before they reach the SPA.
    expect(wrapper.find('.calendar-chip').text()).toContain('PRJ-9')
  })
})

/**
 * The source legend names each source and toggles it off, with the hidden set
 * carried in the URL so a filtered calendar is shareable and survives a reload
 * — the same reasoning as the period and view.
 */
describe('CalendarView source legend', () => {
  const twoSources = [
    { entity: 'task', label: 'Tasks', date: 'due', summary: 'title', color: 'blue', max_span: 31 },
    {
      entity: 'meeting',
      label: 'Meetings',
      date: 'starts_at',
      summary: 'name',
      color: 'violet',
      max_span: 31,
    },
  ]

  function bothSources() {
    return setup({
      sources: twoSources,
      responses: {
        task: [task('T-1', '2026-08-22')],
        meeting: [meeting('M-1', '2026-08-24T09:00:00Z')],
      },
    })
  }

  it('names each source in the legend', async () => {
    const wrapper = bothSources()
    await flushPromises()

    const labels = wrapper.findAll('.calendar-legend-label').map((n) => n.text())
    expect(labels).toEqual(['Tasks', 'Meetings'])
  })

  it('is absent for a single-source calendar, which has nothing to distinguish', async () => {
    const wrapper = setup({ responses: { task: [task('T-1', '2026-08-22')] } })
    await flushPromises()

    expect(wrapper.find('.calendar-legend').exists()).toBe(false)
  })

  it('hides a source’s events and records it in the URL', async () => {
    const wrapper = bothSources()
    await flushPromises()
    expect(wrapper.text()).toContain('Task T-1')

    await wrapper.findAll('.calendar-legend-item')[0].trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('Task T-1')
    expect(wrapper.text()).toContain('Meeting M-1')

    const calls = routerReplace.mock.calls
    expect((calls[calls.length - 1][0].query as Record<string, string>).hide).toBe('0')
  })

  it('restores hidden sources from the URL', async () => {
    const wrapper = setup({
      sources: twoSources,
      query: { hide: '1' },
      responses: {
        task: [task('T-1', '2026-08-22')],
        meeting: [meeting('M-1', '2026-08-24T09:00:00Z')],
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Task T-1')
    expect(wrapper.text()).not.toContain('Meeting M-1')
  })

  it('says sources are hidden rather than claiming the period is empty', async () => {
    const wrapper = setup({
      sources: twoSources,
      query: { hide: '0,1' },
      responses: {
        task: [task('T-1', '2026-08-22')],
        meeting: [meeting('M-1', '2026-08-24T09:00:00Z')],
      },
    })
    await flushPromises()

    expect(wrapper.find('.calendar-empty').text()).toContain('All sources are hidden')
  })

  it('ignores an out-of-range index from a stale link', async () => {
    // The config changed under an old URL. Showing a source the sender had
    // hidden is visibly harmless; erroring is not.
    const wrapper = setup({
      sources: twoSources,
      query: { hide: '7' },
      responses: {
        task: [task('T-1', '2026-08-22')],
        meeting: [meeting('M-1', '2026-08-24T09:00:00Z')],
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Task T-1')
    expect(wrapper.text()).toContain('Meeting M-1')
  })
})

/**
 * Ordering within a day: all-day events first, then timed ones by start time,
 * with the entity id as a stable tiebreak.
 *
 * The tiebreak matters more than it looks: without it two events at the same
 * time have equal sort keys, so they reshuffle between renders as fetches
 * settle — a list that reorders itself while you read it.
 */
describe('CalendarView event ordering', () => {
  it('sorts all-day first, then by start time, then stably by id', async () => {
    const wrapper = setup({
      sources: [
        { entity: 'task', date: 'due', summary: 'title', max_span: 31 },
        { entity: 'meeting', date: 'starts_at', summary: 'name', max_span: 31 },
      ],
      responses: {
        // Deliberately supplied out of order, so passing means the view sorted
        // rather than echoing the fetch order.
        task: [task('T-b', '2026-08-26'), task('T-a', '2026-08-26')],
        meeting: [
          meeting('M-late', '2026-08-26T18:30:00Z'),
          meeting('M-noon-b', '2026-08-26T11:00:00Z'),
          meeting('M-early', '2026-08-26T07:15:00Z'),
          meeting('M-noon-a', '2026-08-26T11:00:00Z'),
        ],
      },
      maxEventsPerDay: 20,
    })
    await flushPromises()

    const titles = wrapper.findAll('.calendar-chip-title').map((n) => n.text())
    expect(titles).toEqual([
      'Task T-a', // all-day, id-sorted
      'Task T-b',
      'Meeting M-early', // 07:15
      'Meeting M-noon-a', // 11:00 — tie broken by id
      'Meeting M-noon-b', // 11:00
      'Meeting M-late', // 18:30
    ])
  })

  it('orders by the DISPLAY timezone, so position agrees with the printed time', async () => {
    // 22:30Z is 00:30 the next day in Amsterdam, so it must not sort as "late
    // on the 26th" — it belongs to the 27th entirely.
    const wrapper = setup({
      sources: [{ entity: 'meeting', date: 'starts_at', summary: 'name', max_span: 31 }],
      responses: {
        meeting: [
          meeting('M-crosses', '2026-08-26T22:30:00Z'),
          meeting('M-morning', '2026-08-27T06:00:00Z'),
        ],
      },
      timezone: 'Europe/Amsterdam',
      maxEventsPerDay: 20,
    })
    await flushPromises()

    // NOT `.find(text === '27')`: a month grid shows spill days, so August
    // 2026 has two cells numbered 27 (July's and August's). Pick the one that
    // actually holds events.
    const cell = wrapper
      .findAll('.calendar-day')
      .find((c) => c.find('.calendar-day-number').text() === '27' && c.find('.calendar-chip').exists())
    const titles = cell!.findAll('.calendar-chip-title').map((n) => n.text())
    // 00:30 sorts before 08:00, both on the 27th in Amsterdam.
    expect(titles).toEqual(['Meeting M-crosses', 'Meeting M-morning'])
  })
})
