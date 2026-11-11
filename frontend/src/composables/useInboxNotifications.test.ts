import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { defineComponent, ref, type Ref } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { Inbox, InboxChannel, InboxEntry, InboxMention } from '@/types/channels'
import {
  DEFAULT_PREFS,
  loadSeen,
  notificationScope,
  support,
  resetNotificationPrefs,
  setPrefs,
} from '@/utils/notificationPrefs'
import { ApiError } from '@/api/errors'
import {
  HIDDEN_DEBOUNCE_MS,
  HIDDEN_MIN_INTERVAL_MS,
  VISIBLE_DEBOUNCE_MS,
  useInboxNotifications,
} from './useInboxNotifications'
import { REFETCH_JITTER_MAX_MS } from './visibleRefetch'

const getInbox = vi.hoisted(() => vi.fn<() => Promise<Inbox>>())
vi.mock('@/api/channels', async (orig) => ({
  ...(await orig<typeof import('@/api/channels')>()),
  getInbox,
}))

const me = vi.hoisted(() => ({ user: 'alice' as string | undefined }))
vi.mock('@/queries/me', async () => {
  const { ref } = await import('vue')
  return { useMe: () => ({ data: ref(me.user ? { user: me.user } : undefined) }) }
})

/** A Web Locks manager: one holder per name, waiters in order, abortable. */
class FakeLocks {
  private held = new Set<string>()
  private waiters = new Map<string, Array<() => void>>()
  requests = 0

  request(name: string, opts: { signal?: AbortSignal }, cb: () => Promise<void> | undefined) {
    this.requests++
    return new Promise<void>((resolve, reject) => {
      const run = () => {
        this.held.add(name)
        Promise.resolve(cb()).finally(() => {
          this.held.delete(name)
          const next = this.waiters.get(name)?.shift()
          next?.()
          resolve()
        })
      }
      if (!this.held.has(name)) return run()
      const queue = this.waiters.get(name) ?? []
      queue.push(run)
      this.waiters.set(name, queue)
      opts.signal?.addEventListener('abort', () => {
        const q = this.waiters.get(name) ?? []
        const i = q.indexOf(run)
        if (i >= 0) q.splice(i, 1)
        reject(new DOMException('aborted', 'AbortError'))
      })
    })
  }
}

interface Shown {
  title: string
  body: string
  tag: string
  onclick: (() => void) | null
  close: () => void
}

let failConstruct = false

let shown: Shown[] = []
let locks: FakeLocks
let visibility: 'visible' | 'hidden' = 'hidden'
let focused = false

function stubNotification() {
  class N {
    static permission: NotificationPermission = 'granted'
    static requestPermission = vi.fn()
    onclick: (() => void) | null = null
    onclose: (() => void) | null = null
    close = vi.fn()
    constructor(title: string, opts: { body: string; tag: string }) {
      if (failConstruct) throw new TypeError('Illegal constructor')
      shown.push(Object.assign(this, { title, body: opts.body, tag: opts.tag }) as unknown as Shown)
    }
  }
  vi.stubGlobal('Notification', N)
}

const chan = (anchor: string): InboxChannel => ({
  anchor,
  entity_type: 'ticket',
  anchor_title: `Title ${anchor}`,
  type: 'chat',
  state: 'open',
})

const mention = (anchor: string, seq: number): InboxMention => ({
  kind: 'mention',
  channel: chan(anchor),
  seq,
  at: '2026-11-10T10:00:00Z',
  message_type: 'text',
  author: { user: 'bob' },
  links: { entity: '/api/v1/x', channel: '/api/v1/y' },
})

const waitingEntry = (anchor: string, generation: number) =>
  ({
    kind: 'waiting',
    channel: chan(anchor),
    since: '2026-11-10T10:00:00Z',
    session: {
      state: 'review',
      label: 'Needs review',
      status: 'waiting',
      generation,
      entered_at: '2026-11-10T10:00:00Z',
    },
  }) as unknown as InboxEntry

const page = (mentions: InboxMention[], entries: InboxEntry[] = []): Inbox => ({
  notice: '',
  degraded: false,
  entries,
  count: entries.length,
  mentions,
  mention_count: mentions.length,
})

const router = createRouter({
  history: createMemoryHistory(),
  routes: [{ path: '/:p(.*)*', component: { render: () => null } }],
})

interface Mounted {
  unmount: () => void
  background: () => (() => void) | null
}
const mounted: Mounted[] = []

function mountNotifier(enabled: Ref<boolean> = ref(true)): Mounted {
  let background: (() => void) | null = null
  const summary = {
    hidden: ref(false),
    setBackground: (fn: (() => void) | null) => {
      background = fn
    },
  }
  const Host = defineComponent({
    setup() {
      useInboxNotifications(enabled, summary)
      return () => null
    },
  })
  const pinia = createPinia()
  setActivePinia(pinia)
  const w = mount(Host, { global: { plugins: [pinia, PiniaColada, router] } })
  const m = { unmount: () => w.unmount(), background: () => background }
  mounted.push(m)
  return m
}

const scope = () => notificationScope('alice')

/** Lets a hidden read run: past the debounce, the floor and the jitter. */
async function elapse() {
  await vi.advanceTimersByTimeAsync(HIDDEN_MIN_INTERVAL_MS + REFETCH_JITTER_MAX_MS)
  await flushPromises()
}

beforeEach(() => {
  vi.useFakeTimers()
  localStorage.clear()
  resetNotificationPrefs()
  shown = []
  failConstruct = false
  me.user = 'alice'
  visibility = 'hidden'
  focused = false
  locks = new FakeLocks()
  Object.defineProperty(navigator, 'locks', { value: locks, configurable: true })
  vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visibility)
  vi.spyOn(document, 'hasFocus').mockImplementation(() => focused)
  vi.spyOn(window, 'focus').mockImplementation(() => {})
  stubNotification()
  getInbox.mockReset().mockResolvedValue(page([mention('T-1', 1)]))
  setPrefs(scope(), { ...DEFAULT_PREFS, enabled: true })
})

afterEach(() => {
  mounted.splice(0).forEach((m) => m.unmount())
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('useInboxNotifications', () => {
  it('does nothing while notifications are off', async () => {
    setPrefs(scope(), { ...DEFAULT_PREFS, enabled: false })
    mountNotifier()
    await flushPromises()
    expect(locks.requests).toBe(0)
    expect(getInbox).not.toHaveBeenCalled()
  })

  it('does nothing without a known user', async () => {
    me.user = 'unknown'
    mountNotifier()
    await flushPromises()
    expect(locks.requests).toBe(0)
  })

  it('does nothing while the channel schema has no channel types', async () => {
    mountNotifier(ref(false))
    await flushPromises()
    expect(locks.requests).toBe(0)
  })

  it('takes the hint while holding the lock, and reads a silent baseline first', async () => {
    const m = mountNotifier()
    await flushPromises()
    expect(m.background()).not.toBeNull()
    expect(getInbox).toHaveBeenCalledTimes(1)
    expect(shown).toEqual([])
    expect(loadSeen(scope())?.ids).toEqual(new Set(['m:ticket/T-1/chat/#1']))
  })

  it('notifies a new mention after a hidden hint, and routes on click', async () => {
    const m = mountNotifier()
    await flushPromises()
    getInbox.mockResolvedValue(page([mention('T-1', 2), mention('T-1', 1)]))
    m.background()!()
    await vi.advanceTimersByTimeAsync(HIDDEN_DEBOUNCE_MS - 1)
    expect(getInbox).toHaveBeenCalledTimes(1)
    await elapse()
    expect(getInbox).toHaveBeenCalledTimes(2)
    expect(shown).toHaveLength(1)
    expect(shown[0]!.title).toBe('Mentioned by bob')
    expect(shown[0]!.body).toBe('ticket: Title T-1')
    expect(shown[0]!.tag).toBe('ticket/T-1/chat/')

    shown[0]!.onclick!()
    await flushPromises()
    expect(window.focus).toHaveBeenCalled()
    expect(router.currentRoute.value.fullPath).toBe('/entity/ticket/T-1?channel=chat%2F&seq=2')
    expect(shown[0]!.close).toHaveBeenCalled()
  })

  it('notifies a waiting session again when it re-enters a state', async () => {
    getInbox.mockResolvedValue(page([], [waitingEntry('T-9', 1)]))
    const m = mountNotifier()
    await flushPromises()
    getInbox.mockResolvedValue(page([], [waitingEntry('T-9', 3)]))
    m.background()!()
    await elapse()
    expect(shown.map((s) => s.title)).toEqual(['Waiting on you: Needs review'])
  })

  it('updates the seen set but stays quiet while a tab has focus', async () => {
    const m = mountNotifier()
    await flushPromises()
    focused = true
    visibility = 'visible'
    getInbox.mockResolvedValue(page([mention('T-1', 2)]))
    m.background()!()
    m.background()!()
    await vi.advanceTimersByTimeAsync(VISIBLE_DEBOUNCE_MS)
    await flushPromises()
    expect(getInbox).toHaveBeenCalledTimes(2)
    expect(shown).toEqual([])
    expect(loadSeen(scope())?.ids.has('m:ticket/T-1/chat/#2')).toBe(true)
  })

  it('lets one tab of several notify, and hands over without repeats', async () => {
    const a = mountNotifier()
    const b = mountNotifier()
    await flushPromises()
    expect(a.background()).not.toBeNull()
    expect(b.background()).toBeNull()
    expect(getInbox).toHaveBeenCalledTimes(1)

    // A new mention arrives while the holder closes.
    getInbox.mockResolvedValue(page([mention('T-1', 2), mention('T-1', 1)]))
    a.unmount()
    await flushPromises()
    expect(b.background()).not.toBeNull()
    expect(shown.map((s) => s.title)).toEqual(['Mentioned by bob'])

    b.background()!()
    await elapse()
    expect(shown).toHaveLength(1)
  })

  it('reads at most once per interval while hidden', async () => {
    const m = mountNotifier()
    await flushPromises()
    for (let i = 0; i < 5; i++) m.background()!()
    await elapse()
    expect(getInbox).toHaveBeenCalledTimes(2)
  })

  it('keeps the seen set when a read fails, and catches up on the next', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    const m = mountNotifier()
    await flushPromises()
    getInbox.mockRejectedValueOnce(new Error('down'))
    m.background()!()
    await elapse()
    expect(loadSeen(scope())?.ids).toEqual(new Set(['m:ticket/T-1/chat/#1']))
    getInbox.mockResolvedValue(page([mention('T-1', 2)]))
    m.background()!()
    await elapse()
    expect(shown).toHaveLength(1)
  })

  it('stops and releases the lock when notifications are turned off', async () => {
    const m = mountNotifier()
    await flushPromises()
    setPrefs(scope(), { ...DEFAULT_PREFS, enabled: false })
    await flushPromises()
    expect(m.background()).toBeNull()
    const b = mountNotifier()
    await flushPromises()
    expect(b.background()).toBeNull()
  })
  it('drops a read that was on its way when the notifier restarted', async () => {
    const m = mountNotifier()
    await flushPromises()
    let answer: (v: Inbox) => void = () => {}
    getInbox.mockImplementationOnce(() => new Promise<Inbox>((r) => (answer = r)))
    m.background()!()
    await elapse()
    expect(getInbox).toHaveBeenCalledTimes(2)

    // Off and on again: a new holding with its own silent baseline.
    setPrefs(scope(), { ...DEFAULT_PREFS, enabled: false })
    await flushPromises()
    getInbox.mockResolvedValue(page([mention('T-1', 1)]))
    setPrefs(scope(), { ...DEFAULT_PREFS, enabled: true })
    await flushPromises()
    expect(getInbox).toHaveBeenCalledTimes(3)

    answer(page([mention('T-1', 9), mention('T-1', 1)]))
    await flushPromises()
    expect(shown).toEqual([])
    expect(loadSeen(scope())?.ids.has('m:ticket/T-1/chat/#9')).toBe(false)
  })

  it('forgets what was seen and closes what it showed when the inbox is refused', async () => {
    const m = mountNotifier()
    await flushPromises()
    getInbox.mockResolvedValue(page([mention('T-1', 2), mention('T-1', 1)]))
    m.background()!()
    await elapse()
    expect(shown).toHaveLength(1)

    getInbox.mockRejectedValue(
      new ApiError('refused', {
        kind: 'http',
        status: 403,
        problem: { type: 'https://rela.dev/errors/forbidden', title: 'refused', status: 403 },
        original: {},
      })
    )
    m.background()!()
    await elapse()
    expect(loadSeen(scope())).toBeNull()
    expect(shown[0]!.close).toHaveBeenCalled()
  })

  it('marks the page unsupported when the browser will not build a notification', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    const m = mountNotifier()
    await flushPromises()
    failConstruct = true
    getInbox.mockResolvedValue(page([mention('T-1', 2), mention('T-1', 1)]))
    m.background()!()
    await elapse()
    expect(support.value).toBe('unsupported')
    expect(m.background()).toBeNull()
  })
})
