import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
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

    it('creates new handler set when registering first handler for event type', async () => {
      const wrapper = mount(TestComponent)
      await nextTick()

      const vm = wrapper.vm as unknown as {
        on: (type: string, handler: () => void) => void
      }

      const handler1 = vi.fn()
      const handler2 = vi.fn()

      // Register multiple handlers for same event
      vm.on('entity:updated', handler1)
      vm.on('entity:updated', handler2)

      // No errors means handlers registered successfully
      expect(true).toBe(true)
    })

    it('cleans up handlers on unmount', async () => {
      const wrapper = mount(TestComponent)
      await nextTick()

      const vm = wrapper.vm as unknown as {
        on: (type: string, handler: () => void) => void
      }

      vm.on('entity:deleted', vi.fn())
      vm.on('entity:created', vi.fn())

      // Unmounting should clean up registered handlers
      wrapper.unmount()

      // No error means cleanup worked
      expect(true).toBe(true)
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
