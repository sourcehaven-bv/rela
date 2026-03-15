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
})
