import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import { useSchemaStore } from '@/stores/schema'
import { _setEntityPluralForTest } from '@/api/entities'
import type { Entity, EntityLease, PresenceResponse } from '@/types'
import type { ViewResponse } from '@/api'

// The detail header shows who else has the face open and who holds a claim
// (TKT-IMBF7U). Presence is per face: on a type with faces the subscription
// names the served face, and the server refuses a bare id there, so the page
// must not subscribe one.

const fetchViewMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: vi.fn().mockResolvedValue([]),
}))

const getPresenceMock =
  vi.fn<(type: string, address: string, world?: string) => Promise<PresenceResponse>>()
vi.mock('@/api/presence', () => ({
  getPresence: (...a: [string, string, string?]) => getPresenceMock(...a),
}))

const subs = new Map<string, () => string | null>()
vi.mock('@/composables/useLiveSubscription', async (orig) => ({
  ...(await orig<typeof import('@/composables/useLiveSubscription')>()),
  useLiveSubscription: (label: string, path: () => string | null) => {
    subs.set(label, path)
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/entity/task/T-1', name: 'entity' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

function view(entry: Partial<Entity>): ViewResponse {
  return {
    entry: {
      id: 'T-1',
      type: 'task',
      _title: 'A task',
      properties: { title: 'A task' },
      _actions: {},
      ...entry,
    },
    sections: [],
  } as ViewResponse
}

describe('EntityDetail presence and claim', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    _setEntityPluralForTest('task', 'tasks')
    _setEntityPluralForTest('policy', 'policies')
    const schema = useSchemaStore()
    schema.entityTypes.set('task', {
      name: 'task',
      label: 'Task',
      properties: { title: { type: 'string', values: null } },
    } as never)
    schema.entityTypes.set('policy', {
      name: 'policy',
      label: 'Policy',
      properties: { title: { type: 'string', values: null } },
      faces: { draft: {}, published: {} },
    } as never)
    fetchViewMock.mockReset()
    getPresenceMock.mockReset()
    getPresenceMock.mockResolvedValue({ viewers: [], more: 0 })
    subs.clear()
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  async function mountDetail(type: string, id: string, resp: ViewResponse) {
    fetchViewMock.mockResolvedValue(resp)
    const w = mount(EntityDetail, {
      props: { entityType: type, entityId: id },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    return w
  }

  it('shows the other viewers in the header', async () => {
    getPresenceMock.mockResolvedValue({
      viewers: [{ user: 'bob@example.com', tool: 'data-entry', since: '2026-10-12T10:00:00Z' }],
      more: 0,
    })
    const w = await mountDetail('task', 'T-1', view({}))

    expect(getPresenceMock).toHaveBeenCalledWith('task', 'T-1', undefined)
    expect(subs.get('entity-presence')?.()).toBe('/api/v1/tasks/T-1/_presence')
    const avatar = w.find('.detail-header .presence-avatar')
    expect(avatar.attributes('title')).toBe('bob@example.com (data-entry)')
  })

  it('shows the claim badge from the view entry', async () => {
    const lease: EntityLease = {
      user: 'bob@example.com',
      tool: 'mcp',
      label: 'agent',
      since: '2026-10-12T10:00:00Z',
      status: '<img src=x onerror=alert(1)>',
    }
    const w = await mountDetail('task', 'T-1', view({ _lease: lease }))
    const badge = w.find('.detail-header .lease-badge')
    expect(badge.text()).toBe('Claimed by bob@example.com (agent)')
    expect(badge.attributes('title')).toContain('<img src=x onerror=alert(1)>')
    expect(w.find('.detail-header img').exists()).toBe(false)
  })

  it('shows neither when nobody views or claims', async () => {
    const w = await mountDetail('task', 'T-1', view({}))
    expect(w.text()).toContain('A task')
    expect(w.find('.header-presence').exists()).toBe(false)
  })

  it('subscribes the served face on a type with faces', async () => {
    await mountDetail(
      'policy',
      'POL-1',
      view({ id: 'POL-1', type: 'policy', _self: '/api/v1/policies/POL-1@draft' })
    )
    expect(subs.get('entity-presence')?.()).toBe('/api/v1/policies/POL-1@draft/_presence')
    expect(getPresenceMock).toHaveBeenCalledWith('policy', 'POL-1@draft', undefined)
  })

  it('does not subscribe a bare id on a type with faces', async () => {
    await mountDetail('policy', 'POL-1', view({ id: 'POL-1', type: 'policy' }))
    expect(subs.get('entity-presence')?.()).toBeNull()
    expect(getPresenceMock).not.toHaveBeenCalled()
  })
})
