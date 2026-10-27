import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h, ref, shallowRef } from 'vue'
import { ApiError } from '@/api/errors'
import type { ChannelTarget } from '@/api/channels'
import type { ChannelHead, ChannelMarkRead, ChannelRead, Message } from '@/types/channels'
import { useMarkRead, MARK_READ_INTERVAL_MS } from './useMarkRead'

// The open channel marks itself read (TKT-2B2AIA AC 15): only with the
// newest page held and the tab visible, at most once a second, and once
// more after a 429's Retry-After.

const mark = vi.fn<(t: ChannelTarget, seq?: number) => Promise<ChannelMarkRead>>()
vi.mock('@/api/channels', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/channels')>()),
  markChannelRead: (t: ChannelTarget, seq?: number) => mark(t, seq),
}))

const room: ChannelTarget = { type: 'ticket', address: 'TKT-1', channelType: 'room' }

function headOf(read?: ChannelRead): ChannelHead {
  return {
    channel: { anchor: 'TKT-1', type: 'room', state: 'open', last_seq: 3, rev: 3, read },
    ephemeral: [],
    typing_now: [],
    _actions: {
      post: [],
      edit_own: false,
      delete_own: false,
      remove: false,
      lifecycle: [],
      typing: false,
      transitions: [],
    },
  }
}

function msgs(...seqs: number[]): Message[] {
  return seqs.map((seq) => ({ seq, rev: seq, type: 'text' }))
}

let visibility: DocumentVisibilityState = 'visible'

function setVisibility(v: DocumentVisibilityState) {
  visibility = v
  document.dispatchEvent(new Event('visibilitychange'))
}

/** Mounts useMarkRead over refs the test moves, as useChannel would. */
function host(start: { read?: ChannelRead; seqs?: number[]; hasNewer?: boolean } = {}) {
  const head = shallowRef<ChannelHead | null>(headOf(start.read))
  const messages = shallowRef<Message[]>(msgs(...(start.seqs ?? [1, 2, 3])))
  const hasNewer = ref(start.hasNewer ?? false)
  const target = shallowRef<ChannelTarget | null>(room)
  const takeRead = vi.fn((r: ChannelRead) => {
    if (head.value) head.value = { ...head.value, channel: { ...head.value.channel, read: r } }
  })
  const w = mount(
    defineComponent({
      setup() {
        useMarkRead({ target, head, messages, hasNewer, takeRead })
        return () => h('div')
      },
    })
  )
  return { head, messages, hasNewer, target, takeRead, w }
}

function answer(seq: number, unread = 0): ChannelMarkRead {
  return { seq, unread, last_seq: seq }
}

function http(status: number, retryAfter?: string) {
  return new ApiError(`HTTP ${status}`, {
    kind: 'http',
    status,
    problem: { type: status === 429 ? 'rate_limited' : 'channel_not_found', title: 'x', status },
    original: { response: { headers: retryAfter ? { 'retry-after': retryAfter } : {} } },
  })
}

describe('useMarkRead', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mark.mockReset()
    visibility = 'visible'
    vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visibility)
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('marks once after load and takes the answer as the marker', async () => {
    mark.mockResolvedValue(answer(3))
    const h = host({ read: { seq: 1, unread: 2 } })
    await flushPromises()
    expect(mark).toHaveBeenCalledTimes(1)
    expect(mark).toHaveBeenCalledWith(room, 3)
    expect(h.takeRead).toHaveBeenCalledWith({ seq: 3, unread: 0 })

    await vi.advanceTimersByTimeAsync(5 * MARK_READ_INTERVAL_MS)
    expect(mark).toHaveBeenCalledTimes(1)
  })

  it('marks nothing when the newest seq is not above the marker', async () => {
    host({ read: { seq: 3, unread: 0 } })
    await flushPromises()
    expect(mark).not.toHaveBeenCalled()
  })

  it('marks a channel without a marker', async () => {
    mark.mockResolvedValue(answer(3))
    host()
    await flushPromises()
    expect(mark).toHaveBeenCalledWith(room, 3)
  })

  it('does not mark while the tab is hidden, and marks when it is shown', async () => {
    visibility = 'hidden'
    mark.mockResolvedValue(answer(3))
    host({ read: { seq: 1, unread: 2 } })
    await flushPromises()
    expect(mark).not.toHaveBeenCalled()

    setVisibility('visible')
    await flushPromises()
    expect(mark).toHaveBeenCalledWith(room, 3)
  })

  it('does not mark when newer messages were dropped', async () => {
    const h = host({ read: { seq: 1, unread: 2 }, hasNewer: true })
    await flushPromises()
    expect(mark).not.toHaveBeenCalled()

    mark.mockResolvedValue(answer(3))
    h.hasNewer.value = false
    await flushPromises()
    expect(mark).toHaveBeenCalledWith(room, 3)
  })

  it('sends at most one mark a second, with the newest seq', async () => {
    mark.mockImplementation((_t, seq) => Promise.resolve(answer(seq!)))
    const h = host({ read: { seq: 1, unread: 2 } })
    await flushPromises()
    expect(mark).toHaveBeenCalledTimes(1)

    h.messages.value = msgs(1, 2, 3, 4)
    await flushPromises()
    h.messages.value = msgs(1, 2, 3, 4, 5)
    await flushPromises()
    expect(mark).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(MARK_READ_INTERVAL_MS)
    expect(mark).toHaveBeenCalledTimes(2)
    expect(mark).toHaveBeenLastCalledWith(room, 5)
  })

  it('after a 429 sends one mark with the newest seq once Retry-After passes', async () => {
    mark.mockRejectedValueOnce(http(429, '3'))
    const h = host({ read: { seq: 1, unread: 2 } })
    await flushPromises()
    expect(mark).toHaveBeenCalledTimes(1)

    mark.mockImplementation((_t, seq) => Promise.resolve(answer(seq!)))
    h.messages.value = msgs(1, 2, 3, 4)
    await flushPromises()
    h.messages.value = msgs(1, 2, 3, 4, 5)
    await vi.advanceTimersByTimeAsync(2900)
    expect(mark).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(200)
    expect(mark).toHaveBeenCalledTimes(2)
    expect(mark).toHaveBeenLastCalledWith(room, 5)
    await vi.advanceTimersByTimeAsync(5 * MARK_READ_INTERVAL_MS)
    expect(mark).toHaveBeenCalledTimes(2)
  })

  it.each([403, 404])('drops a %i and tries again only for a newer message', async (status) => {
    mark.mockRejectedValueOnce(http(status))
    const h = host({ read: { seq: 1, unread: 2 } })
    await flushPromises()
    await vi.advanceTimersByTimeAsync(5 * MARK_READ_INTERVAL_MS)
    expect(mark).toHaveBeenCalledTimes(1)
    expect(h.takeRead).not.toHaveBeenCalled()

    mark.mockResolvedValue(answer(4))
    h.messages.value = msgs(1, 2, 3, 4)
    await vi.advanceTimersByTimeAsync(MARK_READ_INTERVAL_MS)
    expect(mark).toHaveBeenCalledTimes(2)
    expect(mark).toHaveBeenLastCalledWith(room, 4)
  })

  it('drops an answer for a channel that is no longer open', async () => {
    let release!: (a: ChannelMarkRead) => void
    mark.mockReturnValueOnce(new Promise((r) => (release = r)))
    const h = host({ read: { seq: 1, unread: 2 } })
    await flushPromises()
    h.target.value = { ...room, channelType: 'other' }
    h.head.value = null
    await flushPromises()
    release(answer(3))
    await flushPromises()
    expect(h.takeRead).not.toHaveBeenCalled()
  })

  it("an old channel's answer does not let a second mark overlap the new one", async () => {
    let releaseOld!: (a: ChannelMarkRead) => void
    mark.mockReturnValueOnce(new Promise((r) => (releaseOld = r)))
    mark.mockReturnValueOnce(new Promise(() => {}))
    const other = { ...room, channelType: 'other' }
    const h = host({ read: { seq: 1, unread: 2 } })
    await flushPromises()
    h.target.value = other
    await flushPromises()
    expect(mark).toHaveBeenCalledTimes(2)
    expect(mark).toHaveBeenLastCalledWith(other, 3)

    releaseOld(answer(3))
    await flushPromises()
    h.messages.value = msgs(1, 2, 3, 4)
    await vi.advanceTimersByTimeAsync(5 * MARK_READ_INTERVAL_MS)
    expect(mark).toHaveBeenCalledTimes(2)
  })

  it('stops when the component is gone', async () => {
    mark.mockRejectedValueOnce(http(429, '1'))
    const h = host({ read: { seq: 1, unread: 2 } })
    await flushPromises()
    h.w.unmount()
    await vi.advanceTimersByTimeAsync(5 * MARK_READ_INTERVAL_MS)
    expect(mark).toHaveBeenCalledTimes(1)
  })
})
