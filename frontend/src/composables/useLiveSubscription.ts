import { onBeforeUnmount, watch, type MaybeRefOrGetter, toValue } from 'vue'
import { putLiveSubscriptions } from '@/api/live'
import { ApiError } from '@/api/errors'

/**
 * Live-view subscriptions (TKT-HJ5Z9Y), shared across the page like the SSE
 * connection in useEvents. Components register (key, path, onChange); the
 * manager sends the whole set to the stream in one debounced PUT and calls
 * onChange when the stream sends `live:changed` for the key.
 *
 * After a PUT is accepted, onChange also runs once for each key that was
 * added: the server takes its baseline at that moment, so a change between
 * the page's own fetch and the baseline would otherwise go unnoticed. A
 * reconnect re-adds every key for the same reason.
 */

interface Entry {
  path: string
  onChange: () => void
}

/** The server's per-stream limit; entries past it are not sent. */
export const MAX_LIVE_SUBSCRIPTIONS = 16
const PUT_DEBOUNCE_MS = 50

const entries = new Map<string, Entry>()
let conn = ''
// key → path the server accepted for the current connection.
let accepted = new Map<string, string>()
let timer: ReturnType<typeof setTimeout> | null = null
let inFlight = false
let again = false
let reconnect: () => void = () => {}
// 409s in a row. A stream that keeps opening with other credentials than the
// PUT (a proxy adding a header to XHR only) would otherwise reconnect in a
// tight loop, since every `ready` resets the stream's own backoff.
let staleAttempts = 0
let staleTimer: ReturnType<typeof setTimeout> | null = null
const STALE_BASE_MS = 1000
const STALE_MAX_MS = 30_000

function schedule() {
  if (!conn || timer) return
  timer = setTimeout(() => {
    timer = null
    void send()
  }, PUT_DEBOUNCE_MS)
}

async function send() {
  if (!conn) return
  if (inFlight) {
    again = true
    return
  }
  inFlight = true
  const forConn = conn
  const sent = new Map([...entries].slice(0, MAX_LIVE_SUBSCRIPTIONS).map(([k, e]) => [k, e.path]))
  if (entries.size > MAX_LIVE_SUBSCRIPTIONS) {
    console.warn(`live views: ${entries.size} subscriptions, only ${MAX_LIVE_SUBSCRIPTIONS} sent`)
  }
  try {
    await putLiveSubscriptions(
      forConn,
      [...sent].map(([key, path]) => ({ key, path }))
    )
    if (conn !== forConn) return
    staleAttempts = 0
    const previous = accepted
    accepted = sent
    for (const [key, path] of sent) {
      if (previous.get(key) === path) continue
      const e = entries.get(key)
      if (e?.path === path) e.onChange()
    }
  } catch (err) {
    const status = err instanceof ApiError ? err.status : undefined
    if (status === 404 && conn === forConn) {
      // The stream closed; the next `ready` brings a new id and a new PUT.
      conn = ''
      accepted = new Map()
    } else if (status === 409) {
      // The stream was opened with other credentials.
      if (!staleTimer) {
        const delay = Math.min(STALE_BASE_MS * 2 ** staleAttempts++, STALE_MAX_MS)
        staleTimer = setTimeout(() => {
          staleTimer = null
          reconnect()
        }, delay)
      }
    } else {
      console.warn('live views: subscription update failed', err)
    }
  } finally {
    inFlight = false
    if (again) {
      again = false
      schedule()
    }
  }
}

/** Called by useEvents with each `ready` connection id. */
export function setLiveConnection(id: string) {
  conn = id
  accepted = new Map()
  schedule()
}

/** Called by useEvents when the stream closes. */
export function clearLiveConnection() {
  conn = ''
  accepted = new Map()
}

/** Called by useEvents for each `live:changed` frame. */
export function handleLiveChanged(key: string) {
  entries.get(key)?.onChange()
}

/** Lets useEvents supply the reconnect a 409 asks for. */
export function setLiveReconnect(fn: () => void) {
  reconnect = fn
}

/**
 * Registers key → path until unregistered. Registering a key again replaces
 * it. The returned function removes only this registration.
 */
export function registerLive(key: string, path: string, onChange: () => void): () => void {
  const entry: Entry = { path, onChange }
  entries.set(key, entry)
  schedule()
  return () => {
    if (entries.get(key) !== entry) return
    entries.delete(key)
    schedule()
  }
}

let instances = 0

/**
 * Keeps a live subscription for the component's lifetime. `path` may be
 * reactive; null means "nothing to subscribe yet". The key is `label`
 * plus a per-instance number, so a component shown twice on a page keeps two
 * subscriptions; `label` is letters, digits and `-` only.
 */
export function useLiveSubscription(
  label: string,
  path: MaybeRefOrGetter<string | null>,
  onChange: () => void
) {
  const key = `${label}.${++instances}`
  let unregister: (() => void) | null = null
  watch(
    () => {
      // A path that cannot be built (a type the schema has not registered
      // yet) means no live updates, never a broken page.
      try {
        return toValue(path)
      } catch {
        return null
      }
    },
    (p) => {
      unregister?.()
      unregister = p ? registerLive(key, p, onChange) : null
    },
    { immediate: true }
  )
  onBeforeUnmount(() => {
    unregister?.()
    unregister = null
  })
}

/** Resets module state between tests. */
export function resetLiveSubscriptions() {
  entries.clear()
  conn = ''
  accepted = new Map()
  if (timer) clearTimeout(timer)
  timer = null
  inFlight = false
  again = false
  reconnect = () => {}
  staleAttempts = 0
  if (staleTimer) clearTimeout(staleTimer)
  staleTimer = null
}
