import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { PiniaColada, useQueryCache } from '@pinia/colada'
import { defineComponent } from 'vue'
import { ApiError } from '@/api/errors'
import type { Inbox } from '@/types/channels'
import { useInbox } from './useInbox'
import { INBOX_SUMMARY_KEY } from './useInboxSummary'

const getInbox = vi.hoisted(() => vi.fn<() => Promise<Inbox>>())
vi.mock('@/api/channels', async (orig) => ({
  ...(await orig<typeof import('@/api/channels')>()),
  getInbox,
}))

const subs = vi.hoisted(() => [] as { label: string; path: unknown; onChange: () => void }[])
vi.mock('@/composables/useLiveSubscription', () => ({
  useLiveSubscription: (label: string, path: unknown, onChange: () => void) => {
    subs.push({ label, path, onChange })
  },
}))

function page(over: Partial<Inbox> = {}): Inbox {
  return {
    notice: '',
    entries: [],
    count: 1,
    truncated: false,
    degraded: false,
    unread_channels: [],
    unread_count: 2,
    mentions: [],
    mention_count: 3,
    ...over,
  }
}

const mounted: Array<{ unmount: () => void }> = []

function mountIt() {
  let api!: ReturnType<typeof useInbox>
  let cache!: ReturnType<typeof useQueryCache>
  const Host = defineComponent({
    setup() {
      api = useInbox()
      cache = useQueryCache()
      return () => null
    },
  })
  mounted.push(mount(Host, { global: { plugins: [createPinia(), PiniaColada] } }))
  return { api: () => api, cache: () => cache }
}

beforeEach(() => {
  getInbox.mockReset().mockResolvedValue(page())
  subs.length = 0
})

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount())
})

describe('useInbox', () => {
  it('reads the first page, subscribes its path, and refetches on a frame', async () => {
    const { api } = mountIt()
    await flushPromises()
    expect(api().inbox.value?.count).toBe(1)
    expect(api().loading.value).toBe(false)
    expect(subs.map((s) => [s.label, s.path])).toEqual([['inbox', '/api/v1/_inbox']])

    getInbox.mockResolvedValue(page({ count: 5 }))
    subs[0]!.onChange()
    await flushPromises()
    expect(api().inbox.value?.count).toBe(5)
  })

  it('writes the page counts into the summary query', async () => {
    const { cache } = mountIt()
    await flushPromises()
    expect(cache().getQueryData(INBOX_SUMMARY_KEY)).toEqual({
      hidden: false,
      summary: {
        waiting: 1,
        waiting_truncated: false,
        unread_channels: 2,
        mentions: 3,
        degraded: false,
      },
    })
  })

  it('keeps what it holds on a transient error', async () => {
    const { api } = mountIt()
    await flushPromises()
    getInbox.mockRejectedValue(new ApiError('down', { kind: 'network', original: {} }))
    subs[0]!.onChange()
    await flushPromises()
    expect(api().inbox.value?.count).toBe(1)
    expect(api().error.value).toBeInstanceOf(ApiError)
  })

  it('drops what it holds on a refusal', async () => {
    const { api } = mountIt()
    await flushPromises()
    getInbox.mockRejectedValue(new ApiError('gone', { kind: 'http', status: 404, original: {} }))
    subs[0]!.onChange()
    await flushPromises()
    expect(api().inbox.value).toBeNull()
  })

  it('lets the later answer win', async () => {
    let resolveFirst!: (v: Inbox) => void
    getInbox.mockReturnValueOnce(new Promise<Inbox>((r) => (resolveFirst = r)))
    const { api } = mountIt()
    getInbox.mockResolvedValueOnce(page({ count: 9 }))
    void api().refetch()
    await flushPromises()
    resolveFirst(page({ count: 1 }))
    await flushPromises()
    expect(api().inbox.value?.count).toBe(9)
  })
})
