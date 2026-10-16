import { toValue } from 'vue'
import type { QueryCache } from '@pinia/colada'

/**
 * Background refetches wait for a visible tab, then start after a random
 * delay (TASK-550RD).
 *
 * Without this, every open tab refetched its queries the moment a laptop
 * woke: the browser fires `online` in all tabs at once, and the SSE stream of
 * each tab reconnects and invalidates everything. Sixty or more list requests
 * then reached the server in the same second.
 *
 * The rules:
 *
 * - A hidden tab marks queries stale but does not fetch.
 * - When the tab becomes visible, or the network comes back while it is
 *   visible, the stale active queries are refreshed after 0 to
 *   REFETCH_JITTER_MAX_MS. Tabs therefore spread out instead of firing
 *   together.
 *
 * Pinia Colada's own `refetchOnWindowFocus` and `refetchOnReconnect` fire
 * immediately and in hidden tabs too, so main.ts turns them off and installs
 * this instead. Because they are off globally, a query cannot opt out of the
 * refresh on return with `refetchOnWindowFocus: false`: the flush cannot
 * tell that from the global default. A query that must not refetch when the
 * tab returns sets `staleTime: Infinity`, as the configuration and the
 * principal do.
 */

/** Upper bound of the random delay before a deferred refetch, in ms. */
export const REFETCH_JITTER_MAX_MS = 3000

let cache: QueryCache | null = null
let timer: ReturnType<typeof setTimeout> | null = null
// Deferred work by key: a hidden tab that receives the same signal many
// times runs it once when shown.
const waiting = new Map<unknown, () => void>()
let installed: (() => void) | null = null

/** True when the page is hidden (a background tab or a minimized window). */
export function isPageHidden(): boolean {
  return typeof document !== 'undefined' && document.visibilityState === 'hidden'
}

/** A random delay in [0, REFETCH_JITTER_MAX_MS). */
export function refetchJitter(): number {
  return Math.floor(Math.random() * REFETCH_JITTER_MAX_MS)
}

/**
 * Invalidates queries matching `filter`. A visible tab refetches the active
 * ones now; a hidden tab only marks them stale, and the refresh runs when
 * the tab is visible again.
 */
export function invalidateWhenVisible(
  queryCache: QueryCache,
  filter: Parameters<QueryCache['invalidateQueries']>[0]
): Promise<unknown> {
  return queryCache.invalidateQueries(filter, !isPageHidden())
}

/**
 * Runs fn now when the tab is visible. Otherwise it runs after the jittered
 * delay, when the tab becomes visible; a later call with the same key
 * replaces an earlier one that has not run yet.
 */
export function runWhenVisible(key: unknown, fn: () => void) {
  if (!isPageHidden()) {
    fn()
    return
  }
  waiting.set(key, fn)
}

/**
 * Drops work queued by runWhenVisible under key. A component that queued work
 * calls it when it unmounts, so the work does not run against a disposed
 * query when the tab is shown.
 */
export function cancelWhenVisible(key: unknown) {
  waiting.delete(key)
}

function flush() {
  timer = null
  if (isPageHidden()) return
  const queued = [...waiting.values()]
  waiting.clear()
  for (const fn of queued) fn()
  if (!cache) return
  // refresh() fetches only a stale entry, so a query with staleTime Infinity
  // (the principal, the configuration) is left alone, as before.
  for (const entry of cache.getEntries()) {
    if (entry.active && entry.options && toValue(entry.options.enabled) !== false) {
      cache.refresh(entry).catch(() => {})
    }
  }
}

function scheduleFlush() {
  if (isPageHidden() || timer) return
  timer = setTimeout(flush, refetchJitter())
}

/**
 * Installs the visibility and network listeners. Calling it again replaces
 * the previous installation.
 */
export function installVisibleRefetch(queryCache: QueryCache) {
  installed?.()
  cache = queryCache
  const onVisibility = () => {
    if (!isPageHidden()) scheduleFlush()
  }
  document.addEventListener('visibilitychange', onVisibility)
  window.addEventListener('online', scheduleFlush)
  installed = () => {
    document.removeEventListener('visibilitychange', onVisibility)
    window.removeEventListener('online', scheduleFlush)
  }
}

/** Resets module state between tests. */
export function resetVisibleRefetch() {
  installed?.()
  installed = null
  cache = null
  if (timer) clearTimeout(timer)
  timer = null
  waiting.clear()
}
