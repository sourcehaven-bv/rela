import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, nextTick, type Ref } from 'vue'
import { accessRefusal } from '@/api/workspaceAccess'
import { useQueryCache } from '@pinia/colada'
import { useEvents, type SSEConnectionState } from './useEvents'
import { entityKeys } from '@/queries/entities'
import type { ListParams } from '@/types/entity'

// Mock stores
const mockGitFetchStatus = vi.fn().mockResolvedValue(undefined)
const mockEntitiesInvalidateAll = vi.fn()
const mockUIError = vi.fn()

vi.mock('@/stores', () => ({
  useGitStore: () => ({
    fetchStatus: mockGitFetchStatus,
  }),
  useEntitiesStore: () => ({
    invalidateAll: mockEntitiesInvalidateAll,
  }),
  useUIStore: () => ({
    error: mockUIError,
  }),
  useSchemaStore: () => ({
    lists: new Map([
      ['stalled', { entity: 'project', columns: [], refresh_types: ['action', 'project', 'tag'] }],
    ]),
  }),
}))

const mockProbeSessionEnded = vi.fn().mockResolvedValue(undefined)
vi.mock('@/api/workspaceAccess', async () => {
  const { ref } = await import('vue')
  return {
    probeSessionEnded: () => mockProbeSessionEnded(),
    accessRefusal: ref<string | null>(null),
  }
})
const mockAccessRefusal = accessRefusal as unknown as Ref<string | null>

// Wrap the MockEventSource from test/setup.ts so tests can grab the
// instance the composable connected to and drive server-sent events
// through its _emit helper.
interface EmittingSource {
  _emit: (type: string, data?: string, lastEventId?: string) => void
}
const BaseEventSource = globalThis.EventSource as unknown as new (url: string) => EventSource
let lastSource: EmittingSource | null = null
vi.stubGlobal(
  'EventSource',
  class extends BaseEventSource {
    constructor(url: string) {
      super(url)
      lastSource = this as unknown as EmittingSource
    }
  }
)

interface TestVm {
  connect: () => void
  disconnect: () => void
  on: (type: string, handler: (data: unknown) => void) => void
  off: (type: string, handler: (data: unknown) => void) => void
}

describe('useEvents', () => {
  let connectionState: { value: SSEConnectionState }

  // Test component that uses the composable
  const TestComponent = defineComponent({
    setup() {
      const result = useEvents()
      connectionState = result.connectionState as { value: SSEConnectionState }
      return result
    },
    template: '<div>Test</div>',
  })

  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  // The SSE connection is a module-level singleton that can survive from
  // a previous test, with listeners bound to that test's Pinia stores.
  // Recycle it so the listeners close over this test's instances.
  async function mountConnected() {
    const wrapper = mount(TestComponent)
    await nextTick()
    const vm = wrapper.vm as unknown as TestVm
    vm.disconnect()
    vm.connect()
    await vi.runAllTimersAsync()
    await flushPromises()
    return { wrapper, vm }
  }

  it('connects on mount', async () => {
    mount(TestComponent)
    await nextTick()
    await vi.runAllTimersAsync()
    await flushPromises()

    expect(connectionState.value.connected).toBe(true)
    expect(connectionState.value.reconnecting).toBe(false)
    expect(connectionState.value.error).toBeNull()
  })

  it('returns connection state and methods', async () => {
    const wrapper = mount(TestComponent)
    await nextTick()

    const {
      connectionState: state,
      connect,
      disconnect,
    } = wrapper.vm as unknown as {
      connectionState: { value: SSEConnectionState }
      connect: () => void
      disconnect: () => void
    }

    expect(state).toBeDefined()
    expect(connect).toBeDefined()
    expect(disconnect).toBeDefined()
  })

  it('initializes with disconnected state', () => {
    // Before mounting, state should reflect initial values
    mount(TestComponent)

    // Connection happens async, so initially might not be connected
    expect(connectionState.value).toBeDefined()
    expect(typeof connectionState.value.connected).toBe('boolean')
    expect(typeof connectionState.value.reconnecting).toBe('boolean')
  })

  describe('on/off handlers', () => {
    it('registers and unregisters event handlers with on/off', async () => {
      const wrapper = mount(TestComponent)
      await nextTick()

      const vm = wrapper.vm as unknown as {
        on: (type: string, handler: () => void) => void
        off: (type: string, handler: () => void) => void
      }

      const handler = vi.fn()

      // Register handler
      vm.on('entity:changed', handler)

      // Unregister handler
      vm.off('entity:changed', handler)

      // Handler should not be called after removal (no way to trigger without EventSource)
      expect(handler).not.toHaveBeenCalled()
    })

    it('dispatches entity:changed to all registered handlers (type only, no id)', async () => {
      const { vm } = await mountConnected()

      const handler1 = vi.fn()
      const handler2 = vi.fn()
      vm.on('entity:changed', handler1)
      vm.on('entity:changed', handler2)

      lastSource!._emit('entity:changed', JSON.stringify({ type: 'ticket' }))

      expect(handler1).toHaveBeenCalledWith({ type: 'ticket' })
      expect(handler2).toHaveBeenCalledWith({ type: 'ticket' })
    })

    it('cleans up handlers on unmount', async () => {
      const { wrapper, vm } = await mountConnected()

      const handler = vi.fn()
      vm.on('entity:changed', handler)

      wrapper.unmount()
      lastSource!._emit('entity:changed', JSON.stringify({ type: 'ticket' }))

      expect(handler).not.toHaveBeenCalled()
    })

    it('handles off for non-existent handler', async () => {
      const wrapper = mount(TestComponent)
      await nextTick()

      const vm = wrapper.vm as unknown as {
        off: (type: string, handler: () => void) => void
      }

      const handler = vi.fn()

      // Should not throw when removing handler that was never added
      expect(() => vm.off('entity:changed', handler)).not.toThrow()
    })
  })

  describe('cache invalidation (FEAT-XY2D1L)', () => {
    it('invalidates the entity-type query prefix on an entity:changed event', async () => {
      const queryCache = useQueryCache()
      const spy = vi.spyOn(queryCache, 'invalidateQueries')
      await mountConnected()

      lastSource!._emit('entity:changed', JSON.stringify({ type: 'ticket' }))

      expect(mockEntitiesInvalidateAll).toHaveBeenCalledTimes(1)
      expect(spy).toHaveBeenCalledWith({ key: ['entities', 'ticket'] }, true)
    })

    it('refetches the lists whose roll-ups read the changed type (Q8)', async () => {
      const queryCache = useQueryCache()
      const spy = vi.spyOn(queryCache, 'invalidateQueries')
      await mountConnected()

      lastSource!._emit('entity:changed', JSON.stringify({ type: 'tag' }))

      expect(spy).toHaveBeenCalledWith({ key: ['entities', 'tag'] }, true)
      const listCall = spy.mock.calls.find(([filter]) => {
        const key = (filter as { key?: readonly unknown[] }).key
        return key?.length === 3 && key[1] === 'project' && key[2] === 'list'
      })
      expect(listCall).toBeDefined()
      const predicate = (listCall![0] as { predicate: (e: { key: readonly unknown[] }) => boolean })
        .predicate
      const stalled = entityKeys.listParams('project', { list_id: 'stalled' } as ListParams)
      const other = entityKeys.listParams('project', { list_id: 'all_projects' } as ListParams)
      expect(predicate({ key: stalled })).toBe(true)
      expect(predicate({ key: other })).toBe(false)
      expect(predicate({ key: entityKeys.list('project') })).toBe(false)
    })

    it('invalidates per type across successive entity:changed events', async () => {
      const queryCache = useQueryCache()
      const spy = vi.spyOn(queryCache, 'invalidateQueries')
      await mountConnected()

      lastSource!._emit('entity:changed', JSON.stringify({ type: 'risk' }))
      lastSource!._emit('entity:changed', JSON.stringify({ type: 'measure' }))

      expect(spy).toHaveBeenCalledWith({ key: ['entities', 'risk'] }, true)
      expect(spy).toHaveBeenCalledWith({ key: ['entities', 'measure'] }, true)
      expect(mockEntitiesInvalidateAll).toHaveBeenCalledTimes(2)
    })

    it('only marks queries stale while the tab is hidden (TASK-550RD)', async () => {
      const queryCache = useQueryCache()
      const spy = vi.spyOn(queryCache, 'invalidateQueries')
      await mountConnected()
      const visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
      try {
        lastSource!._emit('entity:changed', JSON.stringify({ type: 'ticket' }))
        lastSource!._emit('refresh')
      } finally {
        visibility.mockRestore()
      }

      expect(spy).toHaveBeenCalledWith({ key: ['entities', 'ticket'] }, false)
      expect(spy).toHaveBeenCalledWith({ key: ['entities'] }, false)
    })

    it('invalidates every entity query on refresh', async () => {
      const queryCache = useQueryCache()
      const spy = vi.spyOn(queryCache, 'invalidateQueries')
      await mountConnected()

      lastSource!._emit('refresh')

      expect(mockEntitiesInvalidateAll).toHaveBeenCalledTimes(1)
      expect(spy).toHaveBeenCalledWith({ key: ['entities'] }, true)
      expect(mockGitFetchStatus).toHaveBeenCalled()
    })

    it('dispatches refresh to subscribers so config-derived state can reload', async () => {
      const { vm } = await mountConnected()
      const handler = vi.fn()
      vm.on('refresh', handler)

      lastSource!._emit('refresh')

      expect(handler).toHaveBeenCalledTimes(1)
    })

    it('shows a rejected config reload as an error without invalidating', async () => {
      const queryCache = useQueryCache()
      const spy = vi.spyOn(queryCache, 'invalidateQueries')
      await mountConnected()

      lastSource!._emit(
        'config-error',
        JSON.stringify({
          file: 'data-entry.yaml',
          error: 'invalid data-entry.yaml: action "close": script not found',
        })
      )

      expect(mockUIError).toHaveBeenCalledTimes(1)
      expect(mockUIError.mock.calls[0][0]).toContain('previous version is still in use')
      expect(mockUIError.mock.calls[0][0]).toContain(
        'data-entry.yaml: invalid data-entry.yaml: action "close"'
      )
      expect(spy).not.toHaveBeenCalled()
      expect(mockEntitiesInvalidateAll).not.toHaveBeenCalled()
    })

    it('says which file was applied when a realtime.yaml reload leaves data-entry.yaml behind', async () => {
      await mountConnected()

      lastSource!._emit(
        'config-error',
        JSON.stringify({
          file: 'data-entry.yaml',
          applied: 'realtime.yaml',
          error: 'channels.room: realtime.yaml declares no channel type "room"',
        })
      )

      const text = mockUIError.mock.calls[0][0] as string
      expect(text).toContain(
        'realtime.yaml was applied, but data-entry.yaml does not load against it'
      )
      expect(text).toContain('the previous data-entry.yaml is still in use')
      expect(text).not.toContain('Configuration not reloaded')
    })

    it('still reports a config-error whose payload does not parse', async () => {
      await mountConnected()

      lastSource!._emit('config-error', 'not json')

      expect(mockUIError).toHaveBeenCalledWith(
        'Configuration not reloaded; the previous version is still in use.'
      )
    })
  })

  describe('resume token (TKT-3A1200)', () => {
    function sourceUrl() {
      return (lastSource as unknown as { url: string }).url
    }

    async function reconnect(vm: TestVm) {
      vm.disconnect()
      vm.connect()
      await vi.runAllTimersAsync()
    }

    it('sends the last event id on reconnect', async () => {
      const { vm } = await mountConnected()
      lastSource!._emit('ready', '{}', 'tok-ready')
      lastSource!._emit('entity:changed', JSON.stringify({ type: 'ticket' }), 'tok-change')

      await reconnect(vm)

      expect(sourceUrl()).toContain('?last_event_id=tok-change')
    })

    it('keeps the stored id when an event carries none', async () => {
      const { vm } = await mountConnected()
      lastSource!._emit('ping', '{}', 'tok-ping')
      lastSource!._emit('refresh', 'refresh', '')

      await reconnect(vm)

      expect(sourceUrl()).toContain('?last_event_id=tok-ping')
    })

    it('escapes the token in the query', async () => {
      const { vm } = await mountConnected()
      lastSource!._emit('ready', '{}', 'a&b=c')

      await reconnect(vm)

      expect(sourceUrl()).toContain('?last_event_id=a%26b%3Dc')
    })
  })

  it('asks whether the session ended when the stream fails', async () => {
    await mountConnected()
    const source = lastSource as unknown as EventSource
    source.onerror?.(new Event('error'))
    expect(mockProbeSessionEnded).toHaveBeenCalledTimes(1)
  })

  it('closes the stream and opens no new one while a refusal is up', async () => {
    await mountConnected()
    expect(connectionState.value.connected).toBe(true)
    const source = lastSource

    mockAccessRefusal.value = 'session_ended'
    await nextTick()
    expect(connectionState.value.connected).toBe(false)

    // A reconnect after the stream failed does not open a new stream.
    ;(source as unknown as EventSource).onerror?.(new Event('error'))
    await vi.runAllTimersAsync()
    expect(lastSource).toBe(source)
    expect(connectionState.value.connected).toBe(false)
    mockAccessRefusal.value = null
  })

  describe('disconnect', () => {
    it('can disconnect after connecting', async () => {
      const wrapper = mount(TestComponent)
      await nextTick()
      await vi.runAllTimersAsync()
      await flushPromises()

      expect(connectionState.value.connected).toBe(true)

      const vm = wrapper.vm as unknown as {
        disconnect: () => void
      }

      vm.disconnect()

      expect(connectionState.value.connected).toBe(false)
    })
  })
})
