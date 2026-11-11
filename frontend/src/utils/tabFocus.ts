/**
 * Which tabs of a project have focus, shared over a BroadcastChannel
 * (TKT-48S88RB). The tab that notifies is often hidden while the user reads
 * rela in another tab; it must not notify about what is already on screen.
 *
 * A focused tab repeats its state every HEARTBEAT_MS, and a report older
 * than STALE_MS counts as unfocused, so a tab that crashed while focused
 * does not silence notifications for good.
 */

export const HEARTBEAT_MS = 15_000
export const STALE_MS = 2 * HEARTBEAT_MS

interface Report {
  focused: boolean
  at: number
}

interface FocusMessage {
  id: string
  focused: boolean
  /** Sent by a tab that just started: every peer answers with its state. */
  ask?: boolean
}

export interface TabFocus {
  /** True when this tab or another tab of the project has focus. */
  anyFocused(): boolean
  stop(): void
}

function isFocusMessage(v: unknown): v is FocusMessage {
  if (typeof v !== 'object' || v === null) return false
  const m = v as Record<string, unknown>
  return typeof m.id === 'string' && typeof m.focused === 'boolean'
}

/** Starts reporting and listening on the named channel. */
export function startTabFocus(name: string): TabFocus {
  const selfFocused = () => document.visibilityState === 'visible' && document.hasFocus()
  if (typeof BroadcastChannel === 'undefined') {
    return { anyFocused: selfFocused, stop: () => {} }
  }
  const id = Math.random().toString(36).slice(2)
  const others = new Map<string, Report>()
  const channel = new BroadcastChannel(name)

  // After pagehide the page is going away: its last word is "unfocused".
  // A visibilitychange or focus event that follows must not undo that, or
  // the report would silence notifications until it goes stale.
  let gone = false
  const post = (focused: boolean, ask = false) => {
    if (!gone) channel.postMessage({ id, focused, ...(ask ? { ask } : {}) } satisfies FocusMessage)
  }
  const report = () => post(selfFocused())
  const blur = () => post(false)
  const hide = () => {
    post(false)
    gone = true
  }
  const show = (e: PageTransitionEvent) => {
    // Restored from the back/forward cache: the page is live again.
    if (e.persisted) {
      gone = false
      report()
    }
  }

  channel.onmessage = (e: MessageEvent) => {
    if (!isFocusMessage(e.data) || e.data.id === id) return
    const now = Date.now()
    for (const [k, r] of others) if (now - r.at >= STALE_MS) others.delete(k)
    others.set(e.data.id, { focused: e.data.focused, at: now })
    if (e.data.ask) report()
  }

  window.addEventListener('focus', report)
  window.addEventListener('blur', blur)
  document.addEventListener('visibilitychange', report)
  window.addEventListener('pagehide', hide)
  window.addEventListener('pageshow', show)
  const timer = setInterval(() => {
    if (selfFocused()) post(true)
  }, HEARTBEAT_MS)
  // Ask rather than only tell: a focused peer would otherwise stay unknown
  // until its next heartbeat.
  post(selfFocused(), true)

  return {
    anyFocused() {
      if (selfFocused()) return true
      const now = Date.now()
      for (const r of others.values()) {
        if (r.focused && now - r.at < STALE_MS) return true
      }
      return false
    },
    stop() {
      clearInterval(timer)
      window.removeEventListener('focus', report)
      window.removeEventListener('blur', blur)
      document.removeEventListener('visibilitychange', report)
      window.removeEventListener('pagehide', hide)
      window.removeEventListener('pageshow', show)
      post(false)
      channel.close()
    },
  }
}
