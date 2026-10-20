import { onScopeDispose, ref, shallowRef, watch, type MaybeRefOrGetter, toValue } from 'vue'
import {
  editMessage,
  getChannelHead,
  getMessages,
  postMessage,
  releaseMessageAction,
  removeMessage,
  runLifecycle,
  runMessageAction,
  type ChannelTarget,
  type PostResult,
} from '@/api/channels'
import { liveChannelPath } from '@/api/live'
import { shouldDropHeldContent } from '@/api/errors'
import { useLiveSubscription } from '@/composables/useLiveSubscription'
import type {
  Channel,
  ChannelHead,
  Message,
  MessageActionResponse,
  MessagePage,
  MessagePropInput,
} from '@/types/channels'

/** The most after_rev pages one live frame reads before it reloads instead. */
export const MAX_FOLLOW_PAGES = 10
/**
 * The most messages held. Past it, messages are dropped from the end the
 * user is not reading: the oldest while following live, the newest after
 * loadOlder.
 */
export const MAX_HELD_MESSAGES = 500
/**
 * How long after an `edit_until` or `release_after` deadline the message
 * is read again, so a server clock slightly behind the browser's has
 * passed it too.
 */
export const DEADLINE_SLACK_MS = 1000

export interface UseChannelOptions {
  /**
   * Whether the channel type has ephemeral message types. Their signals
   * live in the head only, so every live frame then refetches the head.
   */
  ephemeral?: MaybeRefOrGetter<boolean>
  /**
   * Called with every head and page the composable takes in, so the caller
   * can reload the channel type descriptor when they name a message type
   * or action it does not know.
   */
  onSeen?: (head: ChannelHead | null, messages: Message[]) => void
}

/**
 * The per-target state of one open channel. A new target gets a new
 * session, so an answer for an old target is recognised and dropped.
 */
interface Session {
  target: ChannelTarget
  /**
   * The rev to follow from. It only moves forward, and only from a page
   * that names next_after_rev; never from a write answer, whose message
   * says nothing of the messages before it. null until the first page.
   */
  heldRev: number | null
  /** The `before` cursor for loadOlder. */
  nextBefore: number | undefined
  /** One cursor fetch at a time; a frame meanwhile sets dirty. */
  running: boolean
  dirty: boolean
  /** Counts full reloads, so a loadOlder answer from before one is dropped. */
  reloads: number
  /** Counts head requests, so only the latest head answer lands. */
  headSeq: number
  loadingOlder: boolean
  /**
   * The `seq:deadline` pairs already read again once, so a server that
   * still names the same deadline (its clock is behind) is not asked in
   * a loop.
   */
  refreshed: Set<string>
}

/**
 * One channel's head and messages, live (TKT-91BK6F).
 *
 * It reads the head and the newest page, then follows the channel by rev:
 * each `live:changed` frame on the head path reads `messages?after_rev=`
 * from the held rev and merges by seq, a newer rev replacing the held copy,
 * so edits, removals and action changes land in place. A frame during a
 * fetch reruns the fetch once after it. A frame that needs more than
 * MAX_FOLLOW_PAGES pages reloads the newest page instead.
 *
 * A write answer merges its message at once; the frame that follows is
 * then a no-op merge. A post the server accepts as an ephemeral signal
 * (202) refetches the head, where signals live.
 *
 * A message's `_actions` can change without a new rev. Two cases are
 * covered:
 * - Time. A message names `edit_until` and `release_after`. One timer runs
 *   for the soonest deadline among the held messages; it reads the page
 *   that ends at that message again, and the copy merges in place.
 * - Policy and lifecycle. A frame that brings no message refetches the
 *   head. When the channel state or the head affordances moved, or the
 *   channel type has no ephemeral messages (so the frame cannot have been
 *   a signal), the newest page is read again too. Older pages the user
 *   loaded keep their affordances until they are read again; the server
 *   still refuses an operation the caller may no longer run.
 *
 * `head.last` follows the newest held message, so it never lags behind
 * the list.
 *
 * A refused refetch (401/403/404, per shouldDropHeldContent) clears the
 * head and the messages: holding them would keep showing a channel the
 * user may no longer read. Any other failure keeps them. The write
 * functions throw the server's error to the caller and never clear.
 *
 * `target` null means nothing to show yet.
 */
export function useChannel(
  target: MaybeRefOrGetter<ChannelTarget | null>,
  options: UseChannelOptions = {}
) {
  const head = shallowRef<ChannelHead | null>(null)
  const messages = shallowRef<Message[]>([])
  const hasOlder = ref(false)
  /**
   * Whether newer messages exist than the newest held: loadOlder dropped
   * them to stay within MAX_HELD_MESSAGES. Live messages are then not
   * appended; loadNewest returns to the live end.
   */
  const hasNewer = ref(false)
  const error = shallowRef<unknown>(null)
  let session: Session | null = null
  let deadlineTimer: ReturnType<typeof setTimeout> | undefined

  function clear() {
    head.value = null
    messages.value = []
    hasOlder.value = false
    hasNewer.value = false
    stopDeadlineTimer()
  }

  function stopDeadlineTimer() {
    if (deadlineTimer !== undefined) clearTimeout(deadlineTimer)
    deadlineTimer = undefined
  }

  /** The deadlines of m that the client must act on, as `seq:deadline` keys. */
  function deadlinesOf(m: Message): { key: string; at: number }[] {
    const out: { key: string; at: number }[] = []
    for (const d of [m.edit_until, m.release_after]) {
      if (!d) continue
      const at = Date.parse(d)
      if (!Number.isNaN(at)) out.push({ key: `${m.seq}:${d}`, at })
    }
    return out
  }

  /** Arms one timer for the soonest deadline not yet read again. */
  function scheduleDeadline(s: Session) {
    stopDeadlineTimer()
    if (!current(s)) return
    let soonest: { seq: number; key: string; at: number } | undefined
    for (const m of messages.value) {
      for (const d of deadlinesOf(m)) {
        if (s.refreshed.has(d.key)) continue
        if (!soonest || d.at < soonest.at) soonest = { seq: m.seq, ...d }
      }
    }
    if (!soonest) return
    const { seq, key } = soonest
    const wait = Math.max(0, soonest.at - Date.now() + DEADLINE_SLACK_MS)
    deadlineTimer = setTimeout(() => {
      deadlineTimer = undefined
      if (!current(s)) return
      s.refreshed.add(key)
      void run(s, (x) => readAround(x, seq))
    }, wait)
  }

  function current(s: Session): boolean {
    return s === session
  }

  function seen(s: Session) {
    syncLast()
    scheduleDeadline(s)
    options.onSeen?.(head.value, messages.value)
  }

  /** Points head.last at the newest held message when that is newer. */
  function syncLast() {
    const h = head.value
    const newest = messages.value[messages.value.length - 1]
    if (!h || !newest || hasNewer.value) return
    const last = h.last
    if (last && (newest.seq < last.seq || (newest.seq === last.seq && newest.rev <= last.rev))) {
      return
    }
    head.value = { ...h, last: newest }
  }

  function takeRev(s: Session, page: MessagePage) {
    if (page.next_after_rev === undefined) return
    s.heldRev = s.heldRev === null ? page.next_after_rev : Math.max(s.heldRev, page.next_after_rev)
  }

  function setChannel(channel: Channel) {
    if (head.value) head.value = { ...head.value, channel }
  }

  /**
   * Merges ms into the held list by seq. A message replaces the held copy
   * when its rev is not older. A seq outside the held range is taken only
   * when nothing exists beyond it on that side: loadOlder brings older
   * messages and loadNewest newer ones. Past MAX_HELD_MESSAGES the oldest
   * are dropped.
   */
  function merge(s: Session, ms: Message[]) {
    if (ms.length === 0) return
    const bySeq = new Map(messages.value.map((m) => [m.seq, m]))
    const first = messages.value[0]?.seq
    const last = messages.value[messages.value.length - 1]?.seq
    let changed = false
    for (const m of ms) {
      const held = bySeq.get(m.seq)
      if (held) {
        if (m.rev >= held.rev && m !== held) {
          bySeq.set(m.seq, m)
          changed = true
        }
        continue
      }
      if (first !== undefined && m.seq < first && hasOlder.value) continue
      if (last !== undefined && m.seq > last && hasNewer.value) continue
      bySeq.set(m.seq, m)
      changed = true
    }
    if (!changed) return
    let next = [...bySeq.values()].sort((a, b) => a.seq - b.seq)
    if (next.length > MAX_HELD_MESSAGES) {
      next = next.slice(next.length - MAX_HELD_MESSAGES)
      hasOlder.value = true
      s.nextBefore = next[0]?.seq
    }
    messages.value = next
  }

  function fail(s: Session, err: unknown) {
    if (!current(s)) return
    error.value = err
    if (shouldDropHeldContent(err)) {
      clear()
      s.heldRev = null
    }
  }

  async function refetchHead(s: Session) {
    const mine = ++s.headSeq
    const got = await getChannelHead(s.target)
    if (current(s) && mine === s.headSeq) head.value = got
  }

  /** Reads the head and the newest page, replacing what is held. */
  async function reload(s: Session) {
    s.reloads++
    const mine = ++s.headSeq
    const [h, page] = await Promise.all([getChannelHead(s.target), getMessages(s.target)])
    if (!current(s)) return
    if (mine === s.headSeq) head.value = h
    messages.value = []
    hasOlder.value = false
    hasNewer.value = false
    merge(s, page.messages)
    hasOlder.value = page.has_more
    s.nextBefore = page.next_before
    takeRev(s, page)
  }

  /** Reads what changed since the held rev; reloads past the page cap. */
  async function follow(s: Session) {
    if (s.heldRev === null) return reload(s)
    let brought = false
    // A channel without a message may not be stored yet, and a lifecycle
    // operation needs a stored channel: the first message moves the head.
    const unstored = head.value?.channel.last_seq === 0
    for (let i = 0; i < MAX_FOLLOW_PAGES; i++) {
      const page = await getMessages(s.target, { afterRev: s.heldRev })
      if (!current(s)) return
      merge(s, page.messages)
      takeRev(s, page)
      brought ||= page.messages.length > 0
      if (!page.has_more) {
        const stateMoved = head.value?.channel.state !== page.channel.state
        const ephemeral = toValue(options.ephemeral) ?? false
        setChannel(page.channel)
        // A frame that brought no message moved the head itself: a
        // signal, a lifecycle change or the caller's affordances.
        if (!brought || stateMoved || ephemeral || unstored) {
          const before = head.value?._actions
          await refetchHead(s)
          if (!current(s)) return
          const actionsMoved = JSON.stringify(before) !== JSON.stringify(head.value?._actions)
          // A policy change moves message affordances without a new rev.
          if (!brought && (stateMoved || actionsMoved || !ephemeral)) await readNewest(s)
        }
        return
      }
    }
    await reload(s)
  }

  /** Reads the newest page again and merges it in place. */
  async function readNewest(s: Session) {
    if (hasNewer.value) return
    const page = await getMessages(s.target)
    if (current(s)) merge(s, page.messages)
  }

  /** Reads the page that ends at seq again and merges it in place. */
  async function readAround(s: Session, seq: number) {
    const page = await getMessages(s.target, { before: seq + 1 })
    if (current(s)) merge(s, page.messages)
  }

  async function run(s: Session, step: (s: Session) => Promise<void>) {
    if (s.running) {
      s.dirty = true
      return
    }
    s.running = true
    try {
      do {
        s.dirty = false
        try {
          await step(s)
          if (!current(s)) return
          error.value = null
          seen(s)
        } catch (err) {
          fail(s, err)
        }
        step = follow
      } while (s.dirty && current(s))
    } finally {
      s.running = false
    }
  }

  /** Follows the channel; what a live frame calls. */
  function refetch(): Promise<void> {
    return session ? run(session, follow) : Promise.resolve()
  }

  /**
   * Loads the page before the oldest held message. Past MAX_HELD_MESSAGES
   * it drops the newest, since the user is reading the old end, and sets
   * hasNewer.
   */
  async function loadOlder(): Promise<void> {
    const s = session
    if (!s || !hasOlder.value || s.nextBefore === undefined || s.loadingOlder) return
    s.loadingOlder = true
    const reloads = s.reloads
    try {
      const page = await getMessages(s.target, { before: s.nextBefore })
      if (!current(s) || reloads !== s.reloads) return
      const first = messages.value[0]?.seq ?? Infinity
      const older = page.messages.filter((m) => m.seq < first)
      let next = [...older, ...messages.value]
      if (next.length > MAX_HELD_MESSAGES) {
        next = next.slice(0, MAX_HELD_MESSAGES)
        hasNewer.value = true
      }
      messages.value = next
      hasOlder.value = page.has_more
      s.nextBefore = page.next_before
      seen(s)
    } catch (err) {
      fail(s, err)
    } finally {
      s.loadingOlder = false
    }
  }

  /** Returns to the live end: reads the head and the newest page again. */
  function loadNewest(): Promise<void> {
    return session ? run(session, reload) : Promise.resolve()
  }

  /** Merges a write answer, without moving the held rev. */
  function takeWrite(s: Session, m: Message) {
    if (!current(s)) return
    merge(s, [m])
    seen(s)
  }

  function requireSession(): Session {
    if (!session) throw new Error('useChannel: no channel target')
    return session
  }

  async function post(
    type: string,
    props: Record<string, MessagePropInput>,
    replyTo?: number
  ): Promise<PostResult> {
    const s = requireSession()
    const unstored = head.value?.channel.last_seq === 0
    const res = await postMessage(s.target, type, props, replyTo)
    // A new message lands at the live end; when the list is not there,
    // go back to it so the user sees what they posted.
    if (res.kind === 'message' && hasNewer.value) await loadNewest()
    else if (res.kind === 'message') {
      takeWrite(s, res.message)
      // The first message stores the channel, which moves its affordances.
      // Through run, so onSeen sees the new head and a frame meanwhile
      // waits rather than fetching the head a second time.
      if (unstored) await run(s, refetchHead)
    } else await refetchHead(s).catch((err) => fail(s, err))
    return res
  }

  async function edit(seq: number, props: Record<string, MessagePropInput>): Promise<Message> {
    const s = requireSession()
    const m = await editMessage(s.target, seq, props)
    takeWrite(s, m)
    return m
  }

  /** Deletes the caller's own message (no reason) or removes one (reason). */
  async function remove(seq: number, reason?: string): Promise<Message> {
    const s = requireSession()
    const m = await removeMessage(s.target, seq, reason)
    takeWrite(s, m)
    return m
  }

  async function runAction(seq: number, action: string): Promise<MessageActionResponse> {
    const s = requireSession()
    const res = await runMessageAction(s.target, seq, action)
    takeWrite(s, res.message)
    return res
  }

  async function release(seq: number): Promise<Message> {
    const s = requireSession()
    const m = await releaseMessageAction(s.target, seq)
    takeWrite(s, m)
    return m
  }

  /**
   * Runs a lifecycle operation. The head is refetched, since the
   * affordances follow the state; null after a delete, which leaves
   * nothing to show.
   */
  async function lifecycle(op: string): Promise<Channel | null> {
    const s = requireSession()
    const channel = await runLifecycle(s.target, op)
    if (!current(s)) return channel
    if (channel === null) {
      clear()
      return null
    }
    setChannel(channel)
    await refetchHead(s).catch((err) => fail(s, err))
    return channel
  }

  const targetKey = () => {
    const t = toValue(target)
    return t ? [t.type, t.address, t.channelType, t.name ?? ''].join('\u0000') : ''
  }
  watch(
    targetKey,
    () => {
      const t = toValue(target)
      // Another channel's messages must not linger while the new one loads.
      clear()
      error.value = null
      session = t
        ? {
            target: { ...t },
            heldRev: null,
            nextBefore: undefined,
            running: false,
            dirty: false,
            reloads: 0,
            headSeq: 0,
            loadingOlder: false,
            refreshed: new Set(),
          }
        : null
      if (session) void run(session, reload)
    },
    { immediate: true }
  )

  // A loop that is still running, or a deadline timer, must not issue
  // requests for a component that is gone.
  onScopeDispose(() => {
    session = null
    stopDeadlineTimer()
  })

  useLiveSubscription(
    'channel',
    () => {
      const t = toValue(target)
      return t ? liveChannelPath(t) : null
    },
    () => void refetch()
  )

  return {
    head,
    messages,
    hasOlder,
    hasNewer,
    error,
    refetch,
    loadOlder,
    loadNewest,
    post,
    edit,
    remove,
    runAction,
    release,
    lifecycle,
  }
}
