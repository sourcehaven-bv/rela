import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { useQueryCache } from '@pinia/colada'
import { useEvents, type SSEConnectionState } from './useEvents'

// Mock stores
const mockGitFetchStatus = vi.fn().mockResolvedValue(undefined)
const mockEntitiesInvalidateAll = vi.fn()

vi.mock('@/stores', () => ({
  useGitStore: () => ({
    fetchStatus: mockGitFetchStatus,
  }),
  useEntitiesStore: () => ({
    invalidateAll: mockEntitiesInvalidateAll,
  }),
}))

// Wrap the MockEventSource from test/setup.ts so tests can grab the
// instance the composable connected to and drive server-sent events
// through its _emit helper.
interface EmittingSource {
  _emit: (type: string, data?: string) => void
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

    const { connectionState: state, connect, disconnect } = wrapper.vm as unknown as {
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
      vm.on('entity:created', handler)

      // Unregister handler
      vm.off('entity:created', handler)

      // Handler should not be called after removal (no way to trigger without EventSource)
      expect(handler).not.toHaveBeenCalled()
    })

    it('dispatches entity events to all registered handlers', async () => {
      const { vm } = await mountConnected()

      const handler1 = vi.fn()
      const handler2 = vi.fn()
      vm.on('entity:updated', handler1)
      vm.on('entity:updated', handler2)

      lastSource!._emit('entity:updated', JSON.stringify({ type: 'ticket', id: 'TKT-9' }))

      expect(handler1).toHaveBeenCalledWith({ type: 'ticket', id: 'TKT-9' })
      expect(handler2).toHaveBeenCalledWith({ type: 'ticket', id: 'TKT-9' })
    })

    it('cleans up handlers on unmount', async () => {
      const { wrapper, vm } = await mountConnected()

      const handler = vi.fn()
      vm.on('entity:deleted', handler)

      wrapper.unmount()
      lastSource!._emit('entity:deleted', JSON.stringify({ type: 'ticket', id: 'TKT-2' }))

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
      expect(() => vm.off('entity:created', handler)).not.toThrow()
    })
  })

  describe('cache invalidation (FEAT-XY2D1L)', () => {
    it('invalidates the entity-type query prefix on entity events', async () => {
      const queryCache = useQueryCache()
      const spy = vi.spyOn(queryCache, 'invalidateQueries')
      await mountConnected()

      lastSource!._emit('entity:updated', JSON.stringify({ type: 'ticket', id: 'TKT-1' }))

      expect(mockEntitiesInvalidateAll).toHaveBeenCalledTimes(1)
      expect(spy).toHaveBeenCalledWith({ key: ['entities', 'ticket'] })
    })

    it('invalidates per event type for created and deleted too', async () => {
      const queryCache = useQueryCache()
      const spy = vi.spyOn(queryCache, 'invalidateQueries')
      await mountConnected()

      lastSource!._emit('entity:created', JSON.stringify({ type: 'risk', id: 'RSK-1' }))
      lastSource!._emit('entity:deleted', JSON.stringify({ type: 'measure', id: 'MSR-1' }))

      expect(spy).toHaveBeenCalledWith({ key: ['entities', 'risk'] })
      expect(spy).toHaveBeenCalledWith({ key: ['entities', 'measure'] })
      expect(mockEntitiesInvalidateAll).toHaveBeenCalledTimes(2)
    })

    it('invalidates every entity query on refresh', async () => {
      const queryCache = useQueryCache()
      const spy = vi.spyOn(queryCache, 'invalidateQueries')
      await mountConnected()

      lastSource!._emit('refresh')

      expect(mockEntitiesInvalidateAll).toHaveBeenCalledTimes(1)
      expect(spy).toHaveBeenCalledWith({ key: ['entities'] })
      expect(mockGitFetchStatus).toHaveBeenCalled()
    })
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
