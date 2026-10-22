import { toValue, watch, type MaybeRefOrGetter } from 'vue'
import { sendTyping, type ChannelTarget } from '@/api/channels'

/** How long to wait after a refused or failed typing call, in ms. */
export const TYPING_BACKOFF_MS = 3000

export interface UseTyping {
  /** Call on every composer `typing` event. Sends when the throttle allows. */
  onInput: () => void
  /**
   * Call before a log post. It stops new sends and resolves once a pending
   * send has settled, so the server sees that send before the post and the
   * post's clear is the last word. It never rejects.
   */
  settle: () => Promise<void>
  /** Call when the post is done, so the next keystroke sends again. */
  reset: () => void
}

/**
 * Tells the server that the user is typing in a channel (POST …/_typing).
 *
 * `onInput` sends only when `allowed()` (the head's `_actions.typing`) and
 * the local throttle has passed. After a 202 the next call waits half of
 * `ttl_seconds`, so the entry is refreshed before it expires; after any
 * error, a 429 included, it waits TYPING_BACKOFF_MS. Errors are never shown:
 * typing is a hint, and a lost hint costs nothing. The throttle compares
 * the browser clock with itself only, never with server time.
 *
 * A log post ends the entry on the server. A typing call still in flight
 * when the post goes out could land after it and show the poster as typing
 * again, so the caller awaits `settle` before posting and calls `reset`
 * after; `reset` also clears the throttle. Another target starts clean too.
 */
export function useTyping(
  target: MaybeRefOrGetter<ChannelTarget>,
  allowed: () => boolean
): UseTyping {
  let nextAt = 0
  let pending: Promise<void> | null = null
  let held = false

  watch(
    () => JSON.stringify(toValue(target)),
    () => {
      nextAt = 0
    }
  )

  function onInput() {
    const now = Date.now()
    if (held || pending || now < nextAt || !allowed()) return
    const sentTo = JSON.stringify(toValue(target))
    pending = sendTyping(toValue(target))
      .then((got) => {
        if (sentTo === JSON.stringify(toValue(target))) nextAt = now + got.ttl_seconds * 500
      })
      .catch(() => {
        if (sentTo === JSON.stringify(toValue(target))) nextAt = now + TYPING_BACKOFF_MS
      })
      .finally(() => {
        pending = null
      })
  }

  return {
    onInput,
    settle: async () => {
      held = true
      await pending
    },
    reset: () => {
      held = false
      nextAt = 0
    },
  }
}
