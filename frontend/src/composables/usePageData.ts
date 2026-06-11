// usePageData — small composable for "load this when the component
// mounts, abort it when the component unmounts" with built-in
// suppression of cancellation errors.
//
// Why this exists:
//   Many components in the SPA fire an axios fetch in onMounted and
//   catch errors with `console.error`. When the user navigates away
//   before the fetch settles, Firefox aborts the underlying request
//   and axios surfaces it as AxiosError(code='ECONNABORTED'|'ERR_CANCELED').
//   Without this composable, every fast back-button in Firefox produces
//   a "Failed to load X" console error and (sometimes) a stale toast.
//   See BUG-6C3V.
//
// Why not VueUse's useAsyncState:
//   VueUse isn't installed in this project. Adding a 90KB dep just for
//   one composable is overkill when the local version is ~30 lines.
//   When/if we adopt VueUse for other reasons, this composable should
//   be replaced by useAsyncState({ resetOnExecute: true }).
//
// Why not just AbortController + onBeforeUnmount everywhere:
//   1. axios.isCancel() does NOT recognise AbortController-driven
//      aborts in axios 1.6.7 — it only matches the legacy CanceledError
//      from CancelToken. We have to inspect err.code instead.
//   2. Browser-driven cancellation (navigation away in Firefox) doesn't
//      go through our AbortController at all — Firefox kills the
//      underlying connection independently and axios surfaces it as
//      ECONNABORTED with our signal.aborted still false. Same code-check
//      applies.
//   Centralising both quirks in one place stops every component author
//   from having to discover and handle this independently.

import { onBeforeUnmount, onMounted } from 'vue'
import { ApiError } from '@/api/errors'

export interface PageDataOptions {
  /** Called once on mount. Receives an AbortSignal that fires on unmount. */
  load: (signal: AbortSignal) => Promise<void>
  /** Called when load() throws a non-cancellation error. Defaults to console.error. */
  onError?: (err: unknown) => void
}

/**
 * isCancelledFetch returns true if the given error represents a fetch
 * cancelled either by our AbortController or by the browser navigating
 * away. Suppresses console noise from both axios CancelToken and modern
 * AbortController paths.
 */
export function isCancelledFetch(err: unknown): boolean {
  if (err == null) return false
  // Errors from the shared client arrive normalized (api/errors.ts).
  if (err instanceof ApiError) return err.kind === 'cancelled'
  // Raw-axios paths (e.g. HelpModal's /api/help fetch) bypass the
  // interceptor. axios surfaces aborts as ECONNABORTED (legacy) or
  // ERR_CANCELED (new); some fetch paths as DOMException name=AbortError.
  const e = err as { code?: string; name?: string; message?: string }
  if (e.code === 'ECONNABORTED' || e.code === 'ERR_CANCELED') return true
  if (e.name === 'AbortError' || e.name === 'CanceledError') return true
  return false
}

/**
 * usePageData wires an async load to the component lifecycle. Aborts
 * the in-flight request on unmount and silences cancellation errors.
 *
 * Usage:
 *   usePageData({
 *     load: async (signal) => {
 *       data.value = await api.fetchSomething({ signal })
 *     },
 *     onError: (err) => uiStore.error('Failed to load something'),
 *   })
 */
export function usePageData(options: PageDataOptions): {
  reload: () => Promise<void>
  abort: () => void
} {
  let controller: AbortController | null = null

  async function reload(): Promise<void> {
    controller?.abort()
    controller = new AbortController()
    const signal = controller.signal
    try {
      await options.load(signal)
    } catch (err) {
      if (isCancelledFetch(err)) return
      if (signal.aborted) return
      if (options.onError) {
        options.onError(err)
      } else {
        console.error(err)
      }
    }
  }

  function abort(): void {
    controller?.abort()
    controller = null
  }

  onMounted(() => {
    void reload()
  })

  onBeforeUnmount(() => {
    abort()
  })

  return { reload, abort }
}
