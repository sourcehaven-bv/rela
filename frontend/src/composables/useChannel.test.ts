import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h, ref } from 'vue'
import { ApiError } from '@/api/errors'
import { registerEntityPlurals } from '@/api/entities'
import type { ChannelTarget, MessagePageQuery, PostResult } from '@/api/channels'
import type { Channel, ChannelHead, Message, MessagePage } from '@/types/channels'
import {
  useChannel,
  MAX_FOLLOW_PAGES,
  MAX_HELD_MESSAGES,
  DEADLINE_SLACK_MS,
  type UseChannelOptions,
} from './useChannel'

// useChannel follows a channel by rev (TKT-91BK6F): the newest page first,
// then `after_rev` pages on each live frame, merged by seq and rev. These
// tests pin the cursor rules of the design review (R13, R17, R18, R19, R27)
// and the held-content rule: a refused refetch clears, a transient one keeps.

const getHead = vi.fn<(t: ChannelTarget) => Promise<ChannelHead>>()
const getPage = vi.fn<(t: ChannelTarget, q?: MessagePageQuery) => Promise<MessagePage>>()
const post = vi.fn<() => Promise<PostResult>>()
const edit = vi.fn<() => Promise<Message>>()
vi.mock('@/api/channels', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/channels')>()),
  getChannelHead: (t: ChannelTarget) => getHead(t),
  getMessages: (t: ChannelTarget, q?: MessagePageQuery) => getPage(t, q),
  postMessage: () => post(),
  editMessage: () => edit(),
  removeMessage: vi.fn(),
  runMessageAction: vi.fn(),
  releaseMessageAction: vi.fn(),
  runLifecycle: vi.fn(),
}))

const subs: { label: string; path: () => string | null; onChange: () => void }[] = []
vi.mock('@/composables/useLiveSubscription', () => ({
  useLiveSubscription: (label: string, path: () => string | null, onChange: () => void) => {
    subs.push({ label, path, onChange })
  },
}))

function channel(over: Partial<Channel> = {}): Channel {
  return { anchor: 'TKT-1', type: 'room', state: 'open', last_seq: 0, rev: 0, ...over }
}

function headOf(over: Partial<Channel> = {}): ChannelHead {
  return {
    channel: channel(over),
    ephemeral: [],
    _actions: { post: ['text'], edit_own: true, delete_own: true, remove: false, lifecycle: [] },
  }
}

function msg(seq: number, rev = seq, body = `m${seq}`): Message {
  return { seq, rev, type: 'text', props: { body } }
}

function page(messages: Message[], over: Partial<MessagePage> = {}): MessagePage {
  return { channel: channel(), messages, has_more: false, ...over }
}

function host(target: () => ChannelTarget | null, options?: UseChannelOptions) {
  let api!: ReturnType<typeof useChannel>
  mount(
    defineComponent({
      setup() {
        api = useChannel(target, options)
        return () => h('div')
      },
    })
  )
  return api
}

function httpError(status: number) {
  return new ApiError(`HTTP ${status}`, { kind: 'http', status, original: null })
}

const room: ChannelTarget = { type: 'ticket', address: 'TKT-1', channelType: 'room' }

function frame() {
  subs[subs.length - 1]!.onChange()
}

function seqs(ms: Message[]) {
  return ms.map((m) => m.seq)
}

describe('useChannel', () => {
  beforeEach(() => {
    registerEntityPlurals(new Map([['ticket', 'tickets']]))
    getHead.mockReset()
    getPage.mockReset()
    post.mockReset()
    edit.mockReset()
    subs.length = 0
    getHead.mockResolvedValue(headOf())
  })

  it('loads the head and the newest page and subscribes the head path', async () => {
    getPage.mockResolvedValueOnce(page([msg(1), msg(2)], { next_after_rev: 2, next_before: 1 }))
    const ch = host(() => ({ ...room, address: 'TKT-1@draft' }))
    await flushPromises()

    expect(getPage).toHaveBeenCalledWith({ ...room, address: 'TKT-1@draft' }, undefined)
    expect(seqs(ch.messages.value)).toEqual([1, 2])
    expect(ch.head.value?.channel.type).toBe('room')
    expect(subs[0]!.label).toBe('channel')
    expect(subs[0]!.path()).toBe('/api/v1/tickets/TKT-1@draft/_channels/room')
  })

  it('follows by rev and merges edits and new messages by seq', async () => {
    getHead.mockResolvedValue(headOf({ last_seq: 2, rev: 2 }))
    getPage.mockResolvedValueOnce(page([msg(1), msg(2)], { next_after_rev: 2 }))
    const ch = host(() => room)
    await flushPromises()
    getHead.mockClear()

    getPage.mockResolvedValueOnce(page([msg(1, 3, 'edited'), msg(3, 4)], { next_after_rev: 4 }))
    frame()
    await flushPromises()

    expect(getPage).toHaveBeenLastCalledWith(room, { afterRev: 2 })
    expect(seqs(ch.messages.value)).toEqual([1, 2, 3])
    expect(ch.messages.value[0]!.props?.body).toBe('edited')
    // R18: a frame that brought messages costs one request.
    expect(getHead).not.toHaveBeenCalled()

    getPage.mockResolvedValueOnce(page([], { next_after_rev: 4 }))
    getPage.mockResolvedValueOnce(page([msg(1, 3, 'edited'), msg(2), msg(3, 4)]))
    frame()
    await flushPromises()
    expect(getPage).toHaveBeenCalledWith(room, { afterRev: 4 })
  })

  it('refetches the head when a frame brings no message', async () => {
    getPage.mockResolvedValueOnce(page([msg(1)], { next_after_rev: 1 }))
    const ch = host(() => room)
    await flushPromises()
    getHead.mockClear()
    getPage.mockClear()

    getHead.mockResolvedValueOnce(headOf({ state: 'closed' }))
    getPage.mockResolvedValueOnce(
      page([], { next_after_rev: 1, channel: channel({ state: 'closed' }) })
    )
    // The state moved, so the newest page is read again for its affordances.
    getPage.mockResolvedValueOnce(page([{ ...msg(1), _actions: [] }]))
    frame()
    await flushPromises()
    expect(getPage).toHaveBeenCalledTimes(2)
    expect(getPage).toHaveBeenLastCalledWith(room, undefined)
    expect(getHead).toHaveBeenCalledTimes(1)
    expect(ch.head.value?.channel.state).toBe('closed')
    expect(ch.messages.value[0]!._actions).toEqual([])
  })

  it('reads the newest page again only when a frame on an ephemeral channel moved state or affordances', async () => {
    getPage.mockResolvedValueOnce(page([msg(1)], { next_after_rev: 1 }))
    const ch = host(() => room, { ephemeral: true })
    await flushPromises()
    getPage.mockClear()

    // A typing signal: nothing but the ephemeral list moved.
    getPage.mockResolvedValueOnce(page([], { next_after_rev: 1 }))
    frame()
    await flushPromises()
    expect(getPage).toHaveBeenCalledTimes(1)

    // A policy change: the head affordances moved.
    getHead.mockResolvedValueOnce({ ...headOf(), _actions: { ...headOf()._actions, post: [] } })
    getPage.mockResolvedValueOnce(page([], { next_after_rev: 1 }))
    getPage.mockResolvedValueOnce(page([{ ...msg(1), _actions: [] }]))
    frame()
    await flushPromises()
    expect(getPage).toHaveBeenCalledTimes(3)
    expect(ch.messages.value[0]!._actions).toEqual([])
  })

  it('reads a message again once its edit or release deadline passes', async () => {
    vi.useFakeTimers()
    try {
      const now = Date.now()
      const until = new Date(now + 60_000).toISOString()
      const after = new Date(now + 120_000).toISOString()
      getPage.mockResolvedValueOnce(
        page(
          [
            { ...msg(1), _actions: ['edit'], edit_until: until },
            { ...msg(2), _actions: [], release_after: after },
          ],
          { next_after_rev: 2 }
        )
      )
      const ch = host(() => room)
      await flushPromises()
      getPage.mockClear()

      getPage.mockResolvedValueOnce(page([{ ...msg(1), _actions: [] }]))
      await vi.advanceTimersByTimeAsync(60_000 + DEADLINE_SLACK_MS)
      expect(getPage).toHaveBeenCalledTimes(1)
      expect(getPage).toHaveBeenLastCalledWith(room, { before: 2 })
      expect(ch.messages.value[0]!._actions).toEqual([])

      getPage.mockResolvedValueOnce(page([msg(1), { ...msg(2), _actions: ['release'] }]))
      await vi.advanceTimersByTimeAsync(60_000)
      expect(getPage).toHaveBeenCalledTimes(2)
      expect(getPage).toHaveBeenLastCalledWith(room, { before: 3 })
      expect(ch.messages.value[1]!._actions).toEqual(['release'])

      // No deadline is left, so no timer runs.
      await vi.advanceTimersByTimeAsync(600_000)
      expect(getPage).toHaveBeenCalledTimes(2)
    } finally {
      vi.useRealTimers()
    }
  })

  it('reads a deadline again only once when the server still names it', async () => {
    vi.useFakeTimers()
    try {
      const until = new Date(Date.now() + 1_000).toISOString()
      const stale = { ...msg(1), _actions: ['edit'], edit_until: until }
      getPage.mockResolvedValueOnce(page([stale], { next_after_rev: 1 }))
      host(() => room)
      await flushPromises()
      getPage.mockClear()

      getPage.mockResolvedValue(page([{ ...stale }]))
      await vi.advanceTimersByTimeAsync(60_000)
      expect(getPage).toHaveBeenCalledTimes(1)
    } finally {
      vi.useRealTimers()
    }
  })

  it('keeps head.last on the newest held message', async () => {
    getPage.mockResolvedValueOnce(page([msg(1)], { next_after_rev: 1 }))
    const ch = host(() => room)
    await flushPromises()
    expect(ch.head.value?.last?.seq).toBe(1)

    getPage.mockResolvedValueOnce(page([msg(2)], { next_after_rev: 2 }))
    frame()
    await flushPromises()
    expect(ch.head.value?.last?.seq).toBe(2)

    getPage.mockResolvedValueOnce(page([msg(2, 3, 'edited')], { next_after_rev: 3 }))
    frame()
    await flushPromises()
    expect(ch.head.value?.last?.props?.body).toBe('edited')
  })

  it('stops after the component is gone', async () => {
    getPage.mockResolvedValueOnce(page([msg(1)], { next_after_rev: 1 }))
    let api!: ReturnType<typeof useChannel>
    const w = mount(
      defineComponent({
        setup() {
          api = useChannel(() => room)
          return () => h('div')
        },
      })
    )
    await flushPromises()
    let release!: (p: MessagePage) => void
    getPage.mockReturnValueOnce(new Promise((r) => (release = r)))
    frame()
    frame()
    await flushPromises()
    w.unmount()
    release(page([msg(2)], { next_after_rev: 2 }))
    await flushPromises()
    expect(getPage).toHaveBeenCalledTimes(2)
    expect(seqs(api.messages.value)).toEqual([1])
  })

  it('refetches the head on every frame for a type with ephemeral messages', async () => {
    getPage.mockResolvedValueOnce(page([msg(1)], { next_after_rev: 1 }))
    host(() => room, { ephemeral: true })
    await flushPromises()
    getHead.mockClear()

    getPage.mockResolvedValueOnce(page([msg(2)], { next_after_rev: 2 }))
    frame()
    await flushPromises()
    expect(getHead).toHaveBeenCalledTimes(1)
  })

  it('runs one fetch at a time and reruns once for frames during it', async () => {
    getPage.mockResolvedValueOnce(page([msg(1)], { next_after_rev: 1 }))
    const ch = host(() => room)
    await flushPromises()

    let release!: (p: MessagePage) => void
    getPage.mockReturnValueOnce(new Promise((r) => (release = r)))
    frame()
    frame()
    frame()
    await flushPromises()
    expect(getPage).toHaveBeenCalledTimes(2)

    getPage.mockResolvedValueOnce(page([msg(3)], { next_after_rev: 3 }))
    release(page([msg(2)], { next_after_rev: 2 }))
    await flushPromises()
    expect(getPage).toHaveBeenCalledTimes(3)
    expect(getPage).toHaveBeenLastCalledWith(room, { afterRev: 2 })
    expect(seqs(ch.messages.value)).toEqual([1, 2, 3])
  })

  it('never moves the held rev back, and an older copy never replaces a newer one', async () => {
    getPage.mockResolvedValueOnce(page([msg(1, 5, 'new')], { next_after_rev: 5 }))
    const ch = host(() => room)
    await flushPromises()

    getPage.mockResolvedValueOnce(page([msg(1, 2, 'old')], { next_after_rev: 2 }))
    frame()
    await flushPromises()
    expect(ch.messages.value[0]!.props?.body).toBe('new')

    getPage.mockResolvedValueOnce(page([], { next_after_rev: 5 }))
    getPage.mockResolvedValueOnce(page([msg(1, 5, 'new')]))
    frame()
    await flushPromises()
    expect(getPage).toHaveBeenCalledWith(room, { afterRev: 5 })
  })

  it('merges a write answer at once without moving the held rev', async () => {
    getPage.mockResolvedValueOnce(page([msg(1)], { next_after_rev: 1 }))
    const ch = host(() => room)
    await flushPromises()

    post.mockResolvedValueOnce({ kind: 'message', message: msg(2, 9) })
    await ch.post('text', { body: 'hi' })
    expect(seqs(ch.messages.value)).toEqual([1, 2])

    getPage.mockResolvedValueOnce(page([msg(2, 9)], { next_after_rev: 9 }))
    frame()
    await flushPromises()
    expect(getPage).toHaveBeenLastCalledWith(room, { afterRev: 1 })
    expect(seqs(ch.messages.value)).toEqual([1, 2])
  })

  it('refetches the head when the first message stores the channel', async () => {
    getPage.mockResolvedValueOnce(page([], { next_after_rev: 0 }))
    const onSeen = vi.fn()
    const ch = host(() => room, { onSeen })
    await flushPromises()
    getHead.mockClear()

    const stored = {
      ...headOf({ last_seq: 1, rev: 1 }),
      _actions: { ...headOf()._actions, lifecycle: ['close'] },
    }
    getHead.mockResolvedValueOnce(stored)
    post.mockResolvedValueOnce({ kind: 'message', message: msg(1) })
    await ch.post('text', { body: 'hi' })
    expect(getHead).toHaveBeenCalledTimes(1)
    expect(ch.head.value?._actions.lifecycle).toEqual(['close'])
    // The refetched head reaches onSeen, so checkNames can read it.
    expect(onSeen).toHaveBeenLastCalledWith(
      expect.objectContaining({ _actions: stored._actions }),
      [msg(1)]
    )

    // Later posts and frames on the stored channel do not read the head.
    getHead.mockClear()
    post.mockResolvedValueOnce({ kind: 'message', message: msg(2) })
    await ch.post('text', { body: 'again' })
    getPage.mockResolvedValueOnce(page([msg(2)], { next_after_rev: 2 }))
    frame()
    await flushPromises()
    expect(getHead).not.toHaveBeenCalled()
  })

  it('refetches the head when a frame brings the first message of the channel', async () => {
    getPage.mockResolvedValueOnce(page([], { next_after_rev: 0 }))
    const ch = host(() => room)
    await flushPromises()
    getHead.mockClear()

    getHead.mockResolvedValueOnce(headOf({ last_seq: 1, rev: 1 }))
    getPage.mockResolvedValueOnce(
      page([msg(1)], { next_after_rev: 1, channel: channel({ last_seq: 1, rev: 1 }) })
    )
    frame()
    await flushPromises()
    expect(getHead).toHaveBeenCalledTimes(1)
    expect(seqs(ch.messages.value)).toEqual([1])
  })

  it('refetches the head after an ephemeral post', async () => {
    getPage.mockResolvedValueOnce(page([], { next_after_rev: 0 }))
    const ch = host(() => room)
    await flushPromises()
    getHead.mockClear()

    const signal = {
      id: 'e1',
      type: 'typing',
      author: { user: 'ann' },
      at: 'x',
      expires_at: 'y',
    }
    getHead.mockResolvedValueOnce({ ...headOf(), ephemeral: [signal] })
    post.mockResolvedValueOnce({ kind: 'ephemeral', accepted: { id: 'e1', expires_at: 'y' } })
    await ch.post('typing', {})
    expect(getHead).toHaveBeenCalledTimes(1)
    expect(ch.head.value?.ephemeral).toEqual([signal])
    expect(ch.messages.value).toEqual([])
  })

  it(`reloads the newest page after ${MAX_FOLLOW_PAGES} full after_rev pages`, async () => {
    getPage.mockResolvedValueOnce(page([msg(1)], { next_after_rev: 1 }))
    const ch = host(() => room)
    await flushPromises()
    getHead.mockClear()
    getPage.mockReset()

    let rev = 1
    getPage.mockImplementation(async (_t, q) => {
      if (q?.afterRev === undefined)
        return page([msg(900)], { next_after_rev: 900, has_more: true })
      rev++
      return page([msg(rev)], { next_after_rev: rev, has_more: true })
    })
    frame()
    await flushPromises()

    const followCalls = getPage.mock.calls.filter(([, q]) => q?.afterRev !== undefined)
    expect(followCalls).toHaveLength(MAX_FOLLOW_PAGES)
    expect(getPage).toHaveBeenLastCalledWith(room, undefined)
    expect(getHead).toHaveBeenCalledTimes(1)
    expect(seqs(ch.messages.value)).toEqual([900])
    expect(ch.hasOlder.value).toBe(true)
  })

  it('drops answers for an old target and clears at once on a switch', async () => {
    const t = ref<ChannelTarget>(room)
    let resolveOld!: (p: MessagePage) => void
    getPage.mockReturnValueOnce(new Promise((r) => (resolveOld = r)))
    const ch = host(() => t.value)
    await flushPromises()

    getPage.mockResolvedValueOnce(page([msg(7)], { next_after_rev: 7 }))
    t.value = { ...room, channelType: 'threads', name: 'x1' }
    await flushPromises()
    expect(seqs(ch.messages.value)).toEqual([7])

    resolveOld(page([msg(1)], { next_after_rev: 1 }))
    await flushPromises()
    expect(seqs(ch.messages.value)).toEqual([7])
  })

  it.each([401, 403, 404])(
    'clears the channel when the refetch is refused with %i',
    async (status) => {
      getPage.mockResolvedValueOnce(page([msg(1)], { next_after_rev: 1 }))
      const ch = host(() => room)
      await flushPromises()

      getPage.mockRejectedValueOnce(httpError(status))
      frame()
      await flushPromises()
      expect(ch.head.value).toBeNull()
      expect(ch.messages.value).toEqual([])
      expect(ch.error.value).toBeInstanceOf(ApiError)

      // The next frame starts over from the newest page.
      getPage.mockResolvedValueOnce(page([msg(2)], { next_after_rev: 2 }))
      frame()
      await flushPromises()
      expect(getPage).toHaveBeenLastCalledWith(room, undefined)
    }
  )

  it('keeps the channel on a transient failure', async () => {
    getPage.mockResolvedValueOnce(page([msg(1)], { next_after_rev: 1 }))
    const ch = host(() => room)
    await flushPromises()

    getPage.mockRejectedValueOnce(httpError(503))
    frame()
    await flushPromises()
    expect(seqs(ch.messages.value)).toEqual([1])
    expect(ch.head.value).not.toBeNull()
    expect(ch.error.value).toBeInstanceOf(ApiError)
  })

  it('loads older messages through next_before', async () => {
    getPage.mockResolvedValueOnce(
      page([msg(5), msg(6)], { next_after_rev: 6, next_before: 5, has_more: true })
    )
    const ch = host(() => room)
    await flushPromises()
    expect(ch.hasOlder.value).toBe(true)

    getPage.mockResolvedValueOnce(page([msg(3), msg(4)], { next_before: 3, has_more: false }))
    await ch.loadOlder()
    expect(getPage).toHaveBeenLastCalledWith(room, { before: 5 })
    expect(seqs(ch.messages.value)).toEqual([3, 4, 5, 6])
    expect(ch.hasOlder.value).toBe(false)
  })

  it(`holds at most ${MAX_HELD_MESSAGES} messages`, async () => {
    const many = Array.from({ length: MAX_HELD_MESSAGES }, (_, i) => msg(i + 1))
    getPage.mockResolvedValueOnce(page(many, { next_after_rev: MAX_HELD_MESSAGES }))
    const ch = host(() => room)
    await flushPromises()
    expect(ch.hasOlder.value).toBe(false)

    const next = MAX_HELD_MESSAGES + 1
    getPage.mockResolvedValueOnce(page([msg(next)], { next_after_rev: next }))
    frame()
    await flushPromises()
    expect(ch.messages.value).toHaveLength(MAX_HELD_MESSAGES)
    expect(ch.messages.value[0]!.seq).toBe(2)
    expect(ch.hasOlder.value).toBe(true)

    getPage.mockResolvedValueOnce(page([msg(1)], { has_more: false }))
    await ch.loadOlder()
    expect(getPage).toHaveBeenLastCalledWith(room, { before: 2 })
  })

  it('drops the newest past the cap after loadOlder and keeps what the user reads', async () => {
    const newest = Array.from({ length: MAX_HELD_MESSAGES }, (_, i) => msg(i + 101))
    getPage.mockResolvedValueOnce(
      page(newest, { next_after_rev: 600, next_before: 101, has_more: true })
    )
    const ch = host(() => room)
    await flushPromises()

    const older = Array.from({ length: 100 }, (_, i) => msg(i + 1))
    getPage.mockResolvedValueOnce(page(older, { has_more: false }))
    await ch.loadOlder()
    expect(ch.messages.value).toHaveLength(MAX_HELD_MESSAGES)
    expect(ch.messages.value[0]!.seq).toBe(1)
    expect(ch.hasNewer.value).toBe(true)

    // A live message does not evict the old end the user is reading.
    getPage.mockResolvedValueOnce(page([msg(601)], { next_after_rev: 601 }))
    frame()
    await flushPromises()
    expect(ch.messages.value[0]!.seq).toBe(1)
    expect(ch.messages.value).toHaveLength(MAX_HELD_MESSAGES)

    getPage.mockResolvedValueOnce(page([msg(600), msg(601)], { next_after_rev: 601 }))
    await ch.loadNewest()
    expect(seqs(ch.messages.value)).toEqual([600, 601])
    expect(ch.hasNewer.value).toBe(false)
  })

  it('reports each head and page to onSeen', async () => {
    const onSeen = vi.fn()
    getPage.mockResolvedValueOnce(page([msg(1)], { next_after_rev: 1 }))
    host(() => room, { onSeen })
    await flushPromises()
    expect(onSeen).toHaveBeenCalledWith(expect.objectContaining({ channel: channel() }), [msg(1)])
  })
})
