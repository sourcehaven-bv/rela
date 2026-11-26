import { onScopeDispose, watch, type MaybeRefOrGetter, toValue } from 'vue'
import { markChannelRead, type ChannelTarget } from '@/api/channels'
import { ApiError } from '@/api/errors'
import { retryAfterSeconds } from '@/components/entity/channels/channelErrors'
import { isPageHidden } from './visibleRefetch'
import type { ChannelHead, ChannelRead, Message } from '@/types/channels'

/** The shortest time between two marks of one channel, in ms. */
export const MARK_READ_INTERVAL_MS = 1000

/** What useMarkRead reads from the open channel, and where it puts the answer. */
export interface MarkReadSource {
  target: MaybeRefOrGetter<ChannelTarget | null>
  head: MaybeRefOrGetter<ChannelHead | null>
  messages: MaybeRefOrGetter<Message[]>
  /** Whether newer messages exist than the newest held (useChannel's hasNewer). */
  hasNewer: MaybeRefOrGetter<boolean>
  /** Takes the server's answer as the held marker. */
  takeRead: (read: ChannelRead) => void
}

/**
 * Marks the open channel read up to its newest held message (TKT-2B2AIA).
 *
 * It marks when all of these hold: the newest page is held (no newer
 * messages were dropped), the tab is visible, and the newest held seq is
 * above the held marker (`read.seq`, 0 when absent). It sends at most one
 * mark per MARK_READ_INTERVAL_MS; a change within that time is sent once
 * when it ends, with the newest seq at that moment. The answer becomes the
 * held marker, which stops the next check.
 *
 * Failures are not shown, since the user did not ask for the mark:
 * - 429 keeps the mark pending and sends it once after `Retry-After`.
 * - Any other failure (403, 404, a network error) gives up on that seq. A
 *   newer message tries again.
 *
 * A hidden tab marks nothing; `visibilitychange` checks again.
 */
export function useMarkRead(src: MarkReadSource) {
  let timer: ReturnType<typeof setTimeout> | undefined
  /**
   * The generation of the mark in flight, or null. A mark for an old target
   * does not block the new one, and its end does not clear the new one's.
   */
  let inFlight: number | null = null
  /** When the last mark was sent, for the interval. */
  let sentAt = -Infinity
  /** When a 429 lets the next mark go. */
  let blockedUntil = 0
  /** The highest seq a failed mark named; it is not sent again. */
  let failedSeq = 0
  /** Counts target changes, so an answer for an old target is dropped. */
  let generation = 0

  /** The seq to mark now, or null when nothing should be marked. */
  function wanted(): number | null {
    if (isPageHidden() || toValue(src.hasNewer) || !toValue(src.target)) return null
    const head = toValue(src.head)
    const ms = toValue(src.messages)
    const newest = ms[ms.length - 1]?.seq
    if (!head || newest === undefined) return null
    if (newest <= (head.channel.read?.seq ?? 0) || newest <= failedSeq) return null
    return newest
  }

  function stopTimer() {
    if (timer !== undefined) clearTimeout(timer)
    timer = undefined
  }

  function check() {
    if (inFlight === generation || timer !== undefined || wanted() === null) return
    const wait = Math.max(sentAt + MARK_READ_INTERVAL_MS, blockedUntil) - Date.now()
    if (wait > 0) {
      timer = setTimeout(() => {
        timer = undefined
        void send()
      }, wait)
      return
    }
    void send()
  }

  async function send() {
    const seq = wanted()
    const t = toValue(src.target)
    if (seq === null || !t) return
    const mine = generation
    inFlight = mine
    sentAt = Date.now()
    try {
      const got = await markChannelRead(t, seq)
      if (mine === generation) src.takeRead({ seq: got.seq, unread: got.unread })
    } catch (err) {
      if (mine !== generation) return
      if (err instanceof ApiError && err.status === 429) {
        blockedUntil = Date.now() + (retryAfterSeconds(err) ?? 1) * 1000
      } else {
        failedSeq = seq
      }
    } finally {
      if (inFlight === mine) inFlight = null
      if (mine === generation) check()
    }
  }

  const targetKey = () => {
    const t = toValue(src.target)
    return t ? [t.type, t.address, t.channelType, t.name ?? ''].join('\u0000') : ''
  }
  watch(targetKey, () => {
    generation++
    stopTimer()
    sentAt = -Infinity
    blockedUntil = 0
    failedSeq = 0
  })

  watch(
    () => {
      const ms = toValue(src.messages)
      const head = toValue(src.head)
      return [
        targetKey(),
        head ? 'head' : '',
        ms[ms.length - 1]?.seq ?? '',
        head?.channel.read?.seq ?? '',
        toValue(src.hasNewer),
      ].join('\u0001')
    },
    check,
    { immediate: true }
  )

  const onVisibility = () => check()
  if (typeof document !== 'undefined') document.addEventListener('visibilitychange', onVisibility)
  onScopeDispose(() => {
    generation++
    stopTimer()
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', onVisibility)
    }
  })
}
