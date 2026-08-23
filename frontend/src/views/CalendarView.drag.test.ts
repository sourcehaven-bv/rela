import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { ref } from 'vue'
import CalendarView from './CalendarView.vue'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

const listAllEntitiesMock = vi.fn()
const updateEntityMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listAllEntities: (...args: unknown[]) => listAllEntitiesMock(...args),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
}))

const routeQuery = ref<Record<string, string>>({ date: '2026-08-22' })
vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    currentRoute: { value: { query: routeQuery.value, path: '/calendar/schedule' } },
  }),
  useRoute: () => ({ query: routeQuery.value, path: '/calendar/schedule' }),
}))

const CAL_ID = 'schedule'

function listResponse(data: Entity[]): ListResponse<Entity> {
  return {
    data,
    meta: { page: 1, per_page: data.length, total: data.length, has_more: false },
  } as ListResponse<Entity>
}

interface DragSetup {
  entities: Entity[]
  source?: Record<string, unknown>
  timezone?: string
  /** Anchor date for the view; defaults to August 2026. */
  date?: string
}

// See CalendarView.test.ts: an un-unmounted view keeps reacting to the shared
// routeQuery ref and refetches during later tests.
const mounted: { unmount: () => void }[] = []

function setup(opts: DragSetup) {
  const pinia = createPinia()
  setActivePinia(pinia)
  routeQuery.value = { date: opts.date ?? '2026-08-22' }

  const schemaStore = useSchemaStore()
  schemaStore.entityTypes.set('task', {
    label: 'Task',
    properties: {
      title: { type: 'string' },
      due: { type: 'date' },
      due_end: { type: 'date' },
    },
  } as never)
  schemaStore.entityTypes.set('meeting', {
    label: 'Meeting',
    properties: {
      name: { type: 'string' },
      starts_at: { type: 'datetime' },
      ends_at: { type: 'datetime' },
    },
  } as never)

  schemaStore.calendars.set(CAL_ID, {
    title: 'Schedule',
    default_view: 'month',
    week_start: 'monday',
    day_start: '08:00',
    day_end: '20:00',
    max_events_per_day: 10,
    sources: [opts.source ?? { entity: 'task', date: 'due', summary: 'title', max_span: 31 }],
  } as never)

  _setEntityPluralForTest('task', 'tasks')
  _setEntityPluralForTest('meeting', 'meetings')
  useUIStore().setDatetimeTimezone(opts.timezone ?? 'UTC')

  listAllEntitiesMock.mockImplementation(() => Promise.resolve(listResponse(opts.entities)))
  updateEntityMock.mockResolvedValue(opts.entities[0])

  const wrapper = mount(CalendarView, {
    props: { id: CAL_ID },
    global: { plugins: [pinia, PiniaColada] },
    attachTo: document.body,
  })
  mounted.push(wrapper)
  return wrapper
}

/** Drag the first chip onto the cell showing `dayNumber`. */
async function dragFirstChipTo(wrapper: ReturnType<typeof mount>, dayNumber: string) {
  const chip = wrapper.find('.calendar-chip')
  await chip.trigger('dragstart', { dataTransfer: { setData: vi.fn(), effectAllowed: '' } })

  const target = wrapper
    .findAll('.calendar-day')
    .find((c) => c.find('.calendar-day-number').text() === dayNumber)
  expect(target, `no cell for day ${dayNumber}`).toBeDefined()
  await target!.trigger('drop', { dataTransfer: { getData: () => '' } })
  await flushPromises()
}

beforeEach(() => {
  listAllEntitiesMock.mockReset()
  updateEntityMock.mockReset()
})

afterEach(() => {
  while (mounted.length) mounted.pop()!.unmount()
  document.body.innerHTML = ''
})

describe('drag to reschedule', () => {
  it('writes the new date for an all-day event', async () => {
    const wrapper = setup({
      entities: [
        { id: 'T-1', type: 'task', properties: { title: 'A', due: '2026-08-22' }, relations: {} },
      ],
    })
    await flushPromises()

    await dragFirstChipTo(wrapper, '25')

    expect(updateEntityMock).toHaveBeenCalledTimes(1)
    const [type, id, patch] = updateEntityMock.mock.calls[0]
    expect(type).toBe('task')
    expect(id).toBe('T-1')
    expect(patch.properties).toEqual({ due: '2026-08-25' })
  })

  /**
   * Start and end move by the same delta in ONE patch. Two writes would leave
   * the entity briefly ending before it starts, and a caller that forgot the
   * end would silently compress the event.
   */
  it('moves start and end together, preserving the span, in a single patch', async () => {
    const wrapper = setup({
      source: { entity: 'task', date: 'due', end_date: 'due_end', summary: 'title', max_span: 31 },
      entities: [
        {
          id: 'T-1',
          type: 'task',
          properties: { title: 'A', due: '2026-08-22', due_end: '2026-08-24' },
          relations: {},
        },
      ],
    })
    await flushPromises()

    await dragFirstChipTo(wrapper, '25')

    expect(updateEntityMock).toHaveBeenCalledTimes(1)
    const patch = updateEntityMock.mock.calls[0][2]
    expect(patch.properties).toEqual({ due: '2026-08-25', due_end: '2026-08-27' })
  })

  it('preserves the wall-clock time of a timed event', async () => {
    const wrapper = setup({
      source: { entity: 'meeting', date: 'starts_at', summary: 'name', max_span: 31 },
      entities: [
        {
          id: 'M-1',
          type: 'meeting',
          properties: { name: 'Standup', starts_at: '2026-08-22T09:00:00Z' },
          relations: {},
        },
      ],
    })
    await flushPromises()

    await dragFirstChipTo(wrapper, '25')

    const patch = updateEntityMock.mock.calls[0][2]
    expect(patch.properties.starts_at).toBe('2026-08-25T09:00:00.000Z')
  })

  /**
   * The DST case that millisecond arithmetic gets wrong: dragging across a
   * spring-forward boundary must leave a 09:00 meeting at 09:00 local, which
   * is a DIFFERENT instant offset either side of the transition.
   */
  it('preserves wall-clock time across a DST boundary', async () => {
    const wrapper = setup({
      date: '2026-03-28',
      source: { entity: 'meeting', date: 'starts_at', summary: 'name', max_span: 31 },
      entities: [
        {
          id: 'M-1',
          type: 'meeting',
          // 09:00 Amsterdam on 28 March (+01:00).
          properties: { name: 'Standup', starts_at: '2026-03-28T08:00:00Z' },
          relations: {},
        },
      ],
      timezone: 'Europe/Amsterdam',
    })
    await flushPromises()

    await dragFirstChipTo(wrapper, '29')

    const patch = updateEntityMock.mock.calls[0][2]
    // Still 09:00 Amsterdam, now at +02:00.
    expect(patch.properties.starts_at).toBe('2026-03-29T07:00:00.000Z')
  })

  it('does not write when the event is dropped on its own day', async () => {
    const wrapper = setup({
      entities: [
        { id: 'T-1', type: 'task', properties: { title: 'A', due: '2026-08-22' }, relations: {} },
      ],
    })
    await flushPromises()

    await dragFirstChipTo(wrapper, '22')

    expect(updateEntityMock).not.toHaveBeenCalled()
  })
})

describe('drag permissions', () => {
  it('marks an entity the principal may not update as non-draggable', async () => {
    const wrapper = setup({
      entities: [
        {
          id: 'T-1',
          type: 'task',
          properties: { title: 'A', due: '2026-08-22' },
          relations: {},
          _actions: { update: false },
        } as never,
      ],
    })
    await flushPromises()

    expect(wrapper.find('.calendar-chip').attributes('draggable')).toBe('false')
  })

  /**
   * Defence in depth: :draggable="false" stops a drag starting from the
   * calendar, but an external drag source can still fire the drop handler, so
   * the write must be refused there too rather than relying on the attribute.
   */
  it('refuses the write even if a drop is forced on a denied entity', async () => {
    const wrapper = setup({
      entities: [
        {
          id: 'T-1',
          type: 'task',
          properties: { title: 'A', due: '2026-08-22' },
          relations: {},
          _actions: { update: false },
        } as never,
      ],
    })
    await flushPromises()

    await dragFirstChipTo(wrapper, '25')

    expect(updateEntityMock).not.toHaveBeenCalled()
  })
})

describe('drag failure handling', () => {
  it('surfaces a server rejection instead of failing silently', async () => {
    const wrapper = setup({
      entities: [
        { id: 'T-1', type: 'task', properties: { title: 'A', due: '2026-08-22' }, relations: {} },
      ],
    })
    await flushPromises()

    const uiStore = useUIStore()
    const errorSpy = vi.spyOn(uiStore, 'error')
    updateEntityMock.mockRejectedValueOnce(new Error('422 unprocessable'))

    await dragFirstChipTo(wrapper, '25')
    await flushPromises()

    expect(errorSpy).toHaveBeenCalled()
  })

  // The unreadable-date guard itself is covered where it lives, as a pure
  // function: calendarGrid.test.ts asserts applyDayDelta returns null for an
  // unparseable value, and the drop handler abandons the write on null rather
  // than sending a patch built from a value it could not read. Reproducing
  // that through a mounted component would mean reaching into <script setup>
  // internals, which tests nothing extra and breaks on any refactor.
})
