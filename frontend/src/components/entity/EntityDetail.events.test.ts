import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import { useSchemaStore } from '@/stores/schema'
import type { Entity } from '@/types'
import type { ViewResponse } from '@/api'

// The detail page reloads its view when the server reports that entities of
// its own type changed. Bursts are debounced to one reload, and nothing
// reloads after unmount.

const fetchViewMock = vi.fn()
type Handler = (data: { type: string }) => void
const handlers = new Set<Handler>()

vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: async () => [],
}))
vi.mock('@/composables/useEvents', async (orig) => ({
  ...(await orig<typeof import('@/composables/useEvents')>()),
  useEvents: () => ({
    on: (event: string, fn: Handler) => {
      if (event === 'entity:changed') handlers.add(fn)
    },
    off: (event: string, fn: Handler) => {
      if (event === 'entity:changed') handlers.delete(fn)
    },
  }),
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/entity/document/DOC-1', name: 'entity' }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

const entityType = 'document'
const view: ViewResponse = {
  entry: {
    id: 'DOC-1',
    type: entityType,
    _title: 'Doc one',
    _self: '/api/v1/documents/DOC-1@concept',
    properties: { title: 'Doc one' },
    content: '',
    _actions: { update: true },
  } as Entity,
  sections: [],
}

describe('EntityDetail entity:changed listener', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    handlers.clear()
    const pinia = createPinia()
    setActivePinia(pinia)
    useSchemaStore().entityTypes.set(entityType, {
      name: entityType,
      label: 'Document',
      properties: { title: { type: 'string', values: null } },
    } as never)
    fetchViewMock.mockReset().mockResolvedValue(view)
  })

  afterEach(() => {
    vi.useRealTimers()
    document.body.innerHTML = ''
  })

  async function mountDetail() {
    const wrapper = mount(EntityDetail, {
      props: { entityType, entityId: 'DOC-1' },
      attachTo: document.body,
      global: { plugins: [createPinia(), PiniaColada] },
    })
    await flushPromises()
    fetchViewMock.mockClear()
    return wrapper
  }

  function emit(type: string) {
    expect(handlers.size).toBeGreaterThan(0)
    for (const fn of handlers) fn({ type })
  }

  it('debounces a burst for its own type into one reload', async () => {
    await mountDetail()
    emit(entityType)
    await vi.advanceTimersByTimeAsync(100)
    emit(entityType)
    await vi.advanceTimersByTimeAsync(100)
    emit(entityType)
    await vi.advanceTimersByTimeAsync(200)
    expect(fetchViewMock).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(100)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(2000)
    console.log("timers",vi.getTimerCount(), fetchViewMock.mock.calls.length)
    expect(fetchViewMock).toHaveBeenCalledTimes(1)
  })

  it('ignores events for another type', async () => {
    await mountDetail()
    emit('topic')
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()
    expect(fetchViewMock).not.toHaveBeenCalled()
  })

  it('does not reload after unmount', async () => {
    const wrapper = await mountDetail()
    emit(entityType)
    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()
    expect(fetchViewMock).not.toHaveBeenCalled()
  })
})
