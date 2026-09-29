import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { defineComponent, reactive, ref, type Ref } from 'vue'
import { useNavStatus } from './useNavStatus'

const getNavStatusMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  getNavStatus: () => getNavStatusMock(),
}))

const route = reactive({ path: '/dashboard' })
vi.mock('vue-router', () => ({ useRoute: () => route }))

// The live-update feed: captured so a test can fire an entity change.
const handlers: Array<() => void> = []
vi.mock('@/composables/useEvents', () => ({
  useEvents: () => ({ on: (_type: string, handler: () => void) => handlers.push(handler) }),
}))

const overdue = { tone: 'error', label: '3 te laat', count: 3 }

// Unmounted after each test, or an earlier host would still be watching the
// shared route and count toward the next test's fetches.
const mounted: Array<{ unmount: () => void }> = []

function mountWith(enabled: Ref<boolean>) {
  let api!: ReturnType<typeof useNavStatus>
  const Host = defineComponent({
    setup() {
      api = useNavStatus(enabled)
      return () => null
    },
  })
  const wrapper = mount(Host, { global: { plugins: [createPinia(), PiniaColada] } })
  mounted.push(wrapper)
  return { wrapper, api: () => api }
}

beforeEach(() => {
  getNavStatusMock.mockReset()
  getNavStatusMock.mockResolvedValue({ items: { '0.0': overdue } })
  handlers.length = 0
  route.path = '/dashboard'
})

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount())
  vi.useRealTimers()
})

describe('useNavStatus', () => {
  it('fetches nothing when no entry declares status rules', async () => {
    mountWith(ref(false))
    await flushPromises()
    expect(getNavStatusMock).not.toHaveBeenCalled()
  })

  it('resolves an entry’s key to its marker', async () => {
    const { api } = mountWith(ref(true))
    await flushPromises()
    expect(api().statusFor('0.0')).toEqual(overdue)
    expect(api().statusFor('0.1')).toBeUndefined()
    expect(api().statusFor(undefined)).toBeUndefined()
  })

  it('recounts on navigation', async () => {
    mountWith(ref(true))
    await flushPromises()
    const before = getNavStatusMock.mock.calls.length

    route.path = '/list/taken'
    await flushPromises()

    expect(getNavStatusMock.mock.calls.length).toBe(before + 1)
  })

  it('recounts once after a burst of entity changes settles', async () => {
    vi.useFakeTimers()
    mountWith(ref(true))
    await flushPromises()
    const before = getNavStatusMock.mock.calls.length

    handlers.forEach((h) => h())
    handlers.forEach((h) => h())
    handlers.forEach((h) => h())
    await vi.advanceTimersByTimeAsync(1000)

    expect(getNavStatusMock.mock.calls.length).toBe(before + 1)
  })
})
