import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import FlyoutList from './FlyoutList.vue'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import { resetFlyout, useFlyout } from '@/composables/useFlyout'
import type { Entity } from '@/types'

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {}, path: '/list/features' }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}))

function bug(id: string, title: string, severity: string): Entity {
  const entity: Entity = {
    id,
    type: 'bug',
    _title: title,
    properties: { title, severity },
    relations: {},
  }
  return entity
}

let pinia: ReturnType<typeof createPinia>
beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  resetFlyout()
  listEntitiesMock.mockReset()
  const schema = useSchemaStore()
  schema.lists.set('bugs', {
    entity: 'bug',
    title: 'Bugs',
    columns: [{ property: 'title' }, { property: 'severity' }],
    filters: [{ property: 'status', operator: '=', value: 'open' }],
  } as never)
  schema.entityTypes.set('bug', {
    name: 'bug',
    properties: {
      title: { type: 'string' },
      severity: { type: 'enum', values: ['low', 'high'] },
    },
  } as never)
})

async function mountList(rows: Entity[], total = rows.length) {
  listEntitiesMock.mockResolvedValue({
    data: rows,
    meta: { total, page: 1, per_page: 50, has_more: total > rows.length },
    included: {},
  })
  const wrapper = mount(FlyoutList, {
    props: { listId: 'bugs' },
    global: { plugins: [pinia, PiniaColada] },
  })
  await flushPromises()
  return wrapper
}

describe('FlyoutList', () => {
  it('reads the list the way its page does', async () => {
    await mountList([])
    const [type, params] = listEntitiesMock.mock.calls[0]
    expect(type).toBe('bug')
    expect(params).toMatchObject({ list_id: 'bugs', 'filter[status][eq]': 'open', page: 1 })
  })

  it('shows each row by title with its enum value as meta', async () => {
    const wrapper = await mountList([bug('B-1', 'Crash on save', 'high')])
    const row = wrapper.find('[data-entity-id="B-1"]')
    expect(row.text()).toContain('Crash on save')
    expect(row.text()).toContain('high')
  })

  it('opens a row in the flyout', async () => {
    const entity = bug('B-1', 'Crash on save', 'high')
    const wrapper = await mountList([entity])
    await wrapper.find('[data-entity-id="B-1"] button').trigger('click')
    expect(useFlyout().entity.value?.id).toBe('B-1')
  })

  it('says how many rows it left out', async () => {
    const wrapper = await mountList([bug('B-1', 'Crash on save', 'high')], 80)
    expect(wrapper.text()).toContain('1 of 80 shown')
  })
})

describe('FlyoutList grouped by date buckets', () => {
  // Noon on Wednesday 2026-06-10 in Amsterdam.
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-06-10T10:00:00Z'))
    useUIStore().setDatetimeTimezone('Europe/Amsterdam')
    const schema = useSchemaStore()
    schema.lists.set('agenda', {
      entity: 'meeting',
      title: 'Agenda',
      columns: [{ property: 'title' }],
      group_by: {
        property: 'at',
        buckets: 'relative',
        labels: { today: 'Vandaag' },
        max_rows: 500,
      },
    } as never)
    schema.entityTypes.set('meeting', {
      name: 'meeting',
      properties: { title: { type: 'string' }, at: { type: 'datetime' } },
    } as never)
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  function meeting(id: string, title: string, at: string): Entity {
    return { id, type: 'meeting', _title: title, properties: { title, at }, relations: {} }
  }

  async function mountAgenda(rows: Entity[]) {
    listEntitiesMock.mockResolvedValue({
      data: rows,
      meta: { total: rows.length, page: 1, per_page: 50, has_more: false },
      included: {},
    })
    const wrapper = mount(FlyoutList, {
      props: { listId: 'agenda' },
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    return wrapper
  }

  it('sorts the read by the date so each bucket arrives together', async () => {
    await mountAgenda([])
    expect(listEntitiesMock.mock.calls[0][1]).toMatchObject({ list_id: 'agenda', sort: 'at' })
  })

  it('shows only the buckets that have rows, under their headings', async () => {
    const wrapper = await mountAgenda([
      meeting('M-1', 'Retro', '2026-06-01T10:00:00Z'),
      meeting('M-2', 'Standup', '2026-06-10T07:05:00Z'),
      meeting('M-3', 'Review', '2026-06-12T10:00:00Z'),
    ])
    const headings = wrapper
      .findAll('[data-section-id]')
      .map((h) => h.attributes('data-section-id'))
    expect(headings).toEqual(['bucket:overdue', 'bucket:today', 'bucket:next_7_days'])
    expect(wrapper.find('[data-section-id="bucket:today"]').text()).toContain('Vandaag')
  })

  it('says when within the bucket: the time today, the weekday this week', async () => {
    const wrapper = await mountAgenda([
      meeting('M-2', 'Standup', '2026-06-10T07:05:00Z'),
      meeting('M-3', 'Review', '2026-06-12T10:00:00Z'),
    ])
    expect(wrapper.find('[data-entity-id="M-2"]').text()).toMatch(/9:05/)
    expect(wrapper.find('[data-entity-id="M-3"]').text()).toContain('Fri')
  })
})
