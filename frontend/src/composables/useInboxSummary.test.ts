import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { PiniaColada, useQueryCache } from '@pinia/colada'
import { defineComponent, ref, type Ref } from 'vue'
import { ApiError } from '@/api/errors'
import type { Inbox, InboxSummary } from '@/types/channels'
import {
  INBOX_SUMMARY_KEY,
  INBOX_SUMMARY_LIVE_KEY,
  readInboxIntoSummary,
  storeInboxSummary,
  useInboxSummary,
  type InboxSummaryState,
} from './useInboxSummary'

const getInboxSummary = vi.hoisted(() => vi.fn<() => Promise<InboxSummary>>())
const getInbox = vi.hoisted(() => vi.fn<() => Promise<Inbox>>())
vi.mock('@/api/channels', async (orig) => ({
  ...(await orig<typeof import('@/api/channels')>()),
  getInboxSummary,
  getInbox,
}))

// registerLive, captured: the key, its path and its onChange.
const live = vi.hoisted(() => ({
  entries: new Map<string, { path: string; onChange: () => void; deliverHidden: boolean }>(),
  registrations: 0,
}))
vi.mock('@/composables/useLiveSubscription', () => ({
  registerLive: (
    key: string,
    path: string,
    onChange: () => void,
    opts: { deliverHidden?: boolean } = {}
  ) => {
    const entry = { path, onChange, deliverHidden: opts.deliverHidden ?? false }
    live.entries.set(key, entry)
    live.registrations++
    return () => {
      if (live.entries.get(key) === entry) live.entries.delete(key)
    }
  },
}))

const refreshHandlers: Array<() => void> = []
vi.mock('@/composables/useEvents', () => ({
  useEvents: () => ({
    on: (type: string, handler: () => void) => {
      if (type === 'refresh') refreshHandlers.push(handler)
    },
  }),
}))

function counts(over: Partial<InboxSummary> = {}): InboxSummary {
  return {
    waiting: 1,
    waiting_truncated: false,
    unread_channels: 0,
    mentions: 2,
    degraded: false,
    ...over,
  }
}

function problem(status: number, type?: string) {
  return new ApiError('refused', {
    kind: 'http',
    status,
    problem: type
      ? { type: `https://rela.dev/errors/${type}`, title: 'refused', status }
      : undefined,
    original: {},
  })
}

const mounted: Array<{ unmount: () => void }> = []

function mountWith(enabled: Ref<boolean>) {
  let api!: ReturnType<typeof useInboxSummary>
  let cache!: ReturnType<typeof useQueryCache>
  const Host = defineComponent({
    setup() {
      api = useInboxSummary(enabled)
      cache = useQueryCache()
      return () => null
    },
  })
  const wrapper = mount(Host, { global: { plugins: [createPinia(), PiniaColada] } })
  mounted.push(wrapper)
  return { wrapper, api: () => api, cache: () => cache }
}

beforeEach(() => {
  getInboxSummary.mockReset().mockResolvedValue(counts())
  live.entries.clear()
  live.registrations = 0
  refreshHandlers.length = 0
})

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount())
})

describe('useInboxSummary', () => {
  it('fetches nothing while the channel schema has no channel types', async () => {
    mountWith(ref(false))
    await flushPromises()
    expect(getInboxSummary).not.toHaveBeenCalled()
    expect(live.entries.size).toBe(0)
  })

  it('registers the fixed key once after the first answer, and refetches on a frame', async () => {
    const { api } = mountWith(ref(true))
    await flushPromises()
    expect(api().summary.value).toEqual(counts())
    expect([...live.entries.keys()]).toEqual([INBOX_SUMMARY_LIVE_KEY])
    expect(live.entries.get(INBOX_SUMMARY_LIVE_KEY)?.path).toBe('/api/v1/_inbox/summary')

    getInboxSummary.mockResolvedValue(counts({ waiting: 0, mentions: 0, unread_channels: 3 }))
    live.entries.get(INBOX_SUMMARY_LIVE_KEY)!.onChange()
    await flushPromises()
    expect(api().summary.value?.unread_channels).toBe(3)
    // Still one registration: a refetch does not move the key's slot.
    expect(live.registrations).toBe(1)
  })

  it('refetches on the app refresh event', async () => {
    mountWith(ref(true))
    await flushPromises()
    const before = getInboxSummary.mock.calls.length
    refreshHandlers.forEach((h) => h())
    await flushPromises()
    expect(getInboxSummary.mock.calls.length).toBe(before + 1)
  })

  it.each([
    { name: 'a network error', err: new ApiError('down', { kind: 'network', original: {} }) },
    { name: 'a 500', err: problem(500) },
    { name: 'a 503 inbox_too_large', err: problem(503, 'inbox_too_large') },
  ])('keeps the last badge on $name', async ({ err }) => {
    const { api } = mountWith(ref(true))
    await flushPromises()
    getInboxSummary.mockRejectedValue(err)
    live.entries.get(INBOX_SUMMARY_LIVE_KEY)!.onChange()
    await flushPromises()
    expect(api().summary.value).toEqual(counts())
    expect(api().hidden.value).toBe(false)
    expect(live.entries.has(INBOX_SUMMARY_LIVE_KEY)).toBe(true)
  })

  it.each([
    { name: 'a 401', err: problem(401) },
    { name: 'a 403 that is not no_principal', err: problem(403, 'forbidden') },
  ])('drops the badge on $name but keeps the item', async ({ err }) => {
    const { api } = mountWith(ref(true))
    await flushPromises()
    getInboxSummary.mockRejectedValue(err)
    live.entries.get(INBOX_SUMMARY_LIVE_KEY)!.onChange()
    await flushPromises()
    expect(api().summary.value).toBeUndefined()
    expect(api().hidden.value).toBe(false)
  })

  it.each([
    { name: 'a 404 (realtime off)', err: problem(404, 'not_found') },
    { name: 'a 403 no_principal', err: problem(403, 'no_principal') },
  ])('hides the item and unregisters on $name', async ({ err }) => {
    const { api } = mountWith(ref(true))
    await flushPromises()
    expect(live.entries.has(INBOX_SUMMARY_LIVE_KEY)).toBe(true)
    getInboxSummary.mockRejectedValue(err)
    live.entries.get(INBOX_SUMMARY_LIVE_KEY)!.onChange()
    await flushPromises()
    expect(api().hidden.value).toBe(true)
    expect(api().summary.value).toBeUndefined()
    expect(live.entries.has(INBOX_SUMMARY_LIVE_KEY)).toBe(false)
  })

  it('never registers when the first answer hides the inbox', async () => {
    getInboxSummary.mockRejectedValue(problem(404, 'not_found'))
    const { api } = mountWith(ref(true))
    await flushPromises()
    expect(api().hidden.value).toBe(true)
    expect(live.registrations).toBe(0)
  })

  it('removes the registration when the owner unmounts', async () => {
    const { wrapper } = mountWith(ref(true))
    await flushPromises()
    wrapper.unmount()
    expect(live.entries.size).toBe(0)
  })

  it('takes the counts a page fetch writes into the cache', async () => {
    const { api, cache } = mountWith(ref(true))
    await flushPromises()
    const page: Inbox = {
      notice: '',
      count: 4,
      truncated: true,
      degraded: true,
      unread_count: 2,
      mention_count: 0,
    }
    storeInboxSummary(cache(), page)
    await flushPromises()
    expect(api().summary.value).toEqual({
      waiting: 4,
      waiting_truncated: true,
      unread_channels: 2,
      mentions: 0,
      degraded: true,
    })
    const state = cache().getQueryData(INBOX_SUMMARY_KEY) as InboxSummaryState
    expect(state.hidden).toBe(false)
  })

  it('hands the hint to a background handler, delivered while hidden, and takes it back', async () => {
    const { api } = mountWith(ref(true))
    await flushPromises()
    const handler = vi.fn()
    api().setBackground(handler)
    const entry = live.entries.get(INBOX_SUMMARY_LIVE_KEY)!
    expect(entry.deliverHidden).toBe(true)
    const before = getInboxSummary.mock.calls.length
    entry.onChange()
    expect(handler).toHaveBeenCalledTimes(1)
    expect(getInboxSummary.mock.calls.length).toBe(before)

    api().setBackground(null)
    const back = live.entries.get(INBOX_SUMMARY_LIVE_KEY)!
    expect(back.deliverHidden).toBe(false)
    back.onChange()
    await flushPromises()
    expect(handler).toHaveBeenCalledTimes(1)
    expect(getInboxSummary.mock.calls.length).toBe(before + 1)
  })

  it('reads the inbox page into the summary', async () => {
    const { api, cache } = mountWith(ref(true))
    await flushPromises()
    getInbox.mockResolvedValue({ notice: '', count: 3, degraded: false, mention_count: 1 })
    const inbox = await readInboxIntoSummary(cache())
    await flushPromises()
    expect(inbox?.count).toBe(3)
    expect(api().summary.value?.waiting).toBe(3)
    expect(api().summary.value?.mentions).toBe(1)
  })

  it.each([
    { name: 'a 404 hides the inbox', err: problem(404), hidden: true },
    { name: 'a no_principal 403 hides the inbox', err: problem(403, 'no_principal'), hidden: true },
    { name: 'a forbidden 403 drops the summary', err: problem(403, 'forbidden'), hidden: false },
  ])('on a page refusal: $name', async ({ err, hidden }) => {
    const { api, cache } = mountWith(ref(true))
    await flushPromises()
    getInbox.mockRejectedValue(err)
    expect(await readInboxIntoSummary(cache())).toBeUndefined()
    await flushPromises()
    expect(api().hidden.value).toBe(hidden)
    expect(api().summary.value).toBeUndefined()
  })

  it('throws on a transient page failure and keeps the badge', async () => {
    const { api, cache } = mountWith(ref(true))
    await flushPromises()
    getInbox.mockRejectedValue(new ApiError('down', { kind: 'network', original: {} }))
    await expect(readInboxIntoSummary(cache())).rejects.toThrow()
    expect(api().summary.value).toEqual(counts())
  })
})
