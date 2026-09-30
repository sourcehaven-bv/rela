import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { defineComponent, reactive, ref, type Ref } from 'vue'
import { ApiError } from '@/api/errors'
import { useNavItems } from './useNavItems'

const getNavItemsMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  getNavItems: () => getNavItemsMock(),
}))

const route = reactive({ path: '/dashboard' })
vi.mock('vue-router', () => ({ useRoute: () => route }))

// The live-update feed: captured so a test can fire an entity change.
const handlers: Array<() => void> = []
vi.mock('@/composables/useEvents', () => ({
  useEvents: () => ({ on: (_type: string, handler: () => void) => handlers.push(handler) }),
}))

const topics = { entries: [{ id: 'TOP-1', type: 'topic', label: 'Website', initial: 'J' }] }

// Unmounted after each test, or an earlier host would still be watching the
// shared route and count toward the next test's fetches.
const mounted: Array<{ unmount: () => void }> = []

function mountWith(enabled: Ref<boolean>) {
  let api!: ReturnType<typeof useNavItems>
  const Host = defineComponent({
    setup() {
      api = useNavItems(enabled)
      return () => null
    },
  })
  const wrapper = mount(Host, { global: { plugins: [createPinia(), PiniaColada] } })
  mounted.push(wrapper)
  return () => api
}

beforeEach(() => {
  getNavItemsMock.mockReset()
  getNavItemsMock.mockResolvedValue({ items: { '1': topics } })
  handlers.length = 0
  route.path = '/dashboard'
})

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount())
  vi.useRealTimers()
})

function httpError(status: number) {
  return new ApiError(`HTTP ${status}`, { kind: 'http', status, original: null })
}

/** Navigates, so the composable refetches, and settles the request. */
async function navigate(path: string) {
  route.path = path
  await flushPromises()
}

describe('useNavItems', () => {
  it('fetches nothing when no group declares items_from', async () => {
    mountWith(ref(false))
    await flushPromises()
    expect(getNavItemsMock).not.toHaveBeenCalled()
  })

  it('resolves a group’s key to its entries', async () => {
    const api = mountWith(ref(true))
    await flushPromises()
    expect(api().itemsFor('1')).toEqual(topics)
    expect(api().itemsFor('2')).toBeUndefined()
  })

  it('refetches once after a burst of entity changes settles', async () => {
    vi.useFakeTimers()
    mountWith(ref(true))
    await flushPromises()
    const before = getNavItemsMock.mock.calls.length

    handlers.forEach((h) => h())
    handlers.forEach((h) => h())
    await vi.advanceTimersByTimeAsync(1000)

    expect(getNavItemsMock.mock.calls.length).toBe(before + 1)
  })

  // The two tests below differ only in the error the refetch carries, and a
  // pass of one says nothing about the other (frontend/CLAUDE.md, "Held
  // content is read-side ACL's blind spot").
  it('drops held entries when the refetch is refused', async () => {
    const api = mountWith(ref(true))
    await flushPromises()
    expect(api().itemsFor('1')).toEqual(topics)

    getNavItemsMock.mockRejectedValue(httpError(403))
    await navigate('/list/taken')

    expect(api().itemsFor('1')).toBeUndefined()
  })

  it('keeps held entries on a transient failure', async () => {
    const api = mountWith(ref(true))
    await flushPromises()

    getNavItemsMock.mockRejectedValue(httpError(503))
    await navigate('/list/taken')

    expect(api().itemsFor('1')).toEqual(topics)
  })
})
