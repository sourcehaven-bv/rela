import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityList from './EntityList.vue'
import { withPageHeader } from '@/composables/pageHeaderTestHost'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, ListResponse } from '@/types'

// A list row carries `_lease` while someone holds a claim on it, and the
// title cell shows who, beside the world badge (TKT-IMBF7U).

const listEntitiesMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  listEntities: (...args: unknown[]) => listEntitiesMock(...args),
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/list/tasks', name: 'list' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

const rows: Entity[] = [
  {
    id: 'T-1',
    type: 'task',
    properties: { title: 'Claimed task' },
    _lease: {
      user: 'bob@example.com',
      tool: 'mcp',
      label: 'agent-1',
      since: '2026-10-12T10:00:00Z',
      status: '<b>fixing</b>',
    },
  },
  { id: 'T-2', type: 'task', properties: { title: 'Free task' } },
]

describe('EntityList claim badge', () => {
  let pinia: ReturnType<typeof createPinia>
  const mounted: VueWrapper[] = []

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest('task', 'tasks')
    const schema = useSchemaStore()
    schema.lists.set('tasks', {
      id: 'tasks',
      title: 'Tasks',
      entity: 'task',
      columns: [{ property: 'title', label: 'Title' }],
    } as never)
    schema.entityTypes.set('task', {
      name: 'task',
      label: 'Task',
      properties: { title: { type: 'string', values: null } },
    } as never)
    listEntitiesMock.mockReset()
    listEntitiesMock.mockResolvedValue({
      data: rows,
      meta: { total: rows.length, page: 1, per_page: 25, has_more: false },
      included: {},
    } as ListResponse<Entity>)
  })

  afterEach(() => {
    for (const w of mounted.splice(0)) w.unmount()
    document.body.innerHTML = ''
  })

  it('shows the badge on the claimed row only, with the status as text', async () => {
    const w = mount(withPageHeader(EntityList), {
      props: { listId: 'tasks' },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    mounted.push(w)
    await flushPromises()

    expect(w.text()).toContain('Free task')
    const badges = w.findAll('.lease-badge')
    expect(badges).toHaveLength(1)
    expect(badges[0]!.find('.lease-badge__text').text()).toBe('Claimed by bob@example.com (agent-1)')
    expect(badges[0]!.attributes('title')).toContain('<b>fixing</b>')
    expect(badges[0]!.find('b').exists()).toBe(false)
  })
})
