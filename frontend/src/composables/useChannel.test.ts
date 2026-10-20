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
    const ch = host(() => ({ ...room, address: 'TKT-1@draft', world: 'main' }))
    await flushPromises()

    expect(getPage).toHaveBeenCalledWith(
      { ...room, address: 'TKT-1@draft', world: 'main' },
      undefined
    )
    expect(seqs(ch.messages.value)).toEqual([1, 2])
    expect(ch.head.value?.channel.type).toBe('room')
    expect(subs[0]!.label).toBe('channel')
    expect(subs[0]!.path()).toBe('/api/v1/tickets/TKT-1@draft/_channels/room?world=main')
  })

  it('follows by rev and merges edits and new messages by seq', async () => {
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
    frame()
    await flushPromises()
    expect(getPage).toHaveBeenLastCalledWith(room, { afterRev: 4 })
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
    frame()
    await flushPromises()
    expect(getPage).toHaveBeenCalledTimes(1)
    expect(getHead).toHaveBeenCalledTimes(1)
    expect(ch.head.value?.channel.state).toBe('closed')
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
    frame()
    await flushPromises()
    expect(getPage).toHaveBeenLastCalledWith(room, { afterRev: 5 })
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

  it('reports each head and page to onSeen', async () => {
    const onSeen = vi.fn()
    getPage.mockResolvedValueOnce(page([msg(1)], { next_after_rev: 1 }))
    host(() => room, { onSeen })
    await flushPromises()
    expect(onSeen).toHaveBeenCalledWith(expect.objectContaining({ channel: channel() }), [msg(1)])
  })
})
