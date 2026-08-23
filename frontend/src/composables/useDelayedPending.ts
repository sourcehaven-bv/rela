import { computed, onScopeDispose, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'

// Anti-flash gate for pending indicators (TKT-TFSNBY).
//
// rela normally talks to a local or well-connected server, so the common
// case resolves well under 100ms and the occasional slow case is 1-2s. An
// indicator wired straight to a boolean therefore spends most of its life
// flashing for a few frames. This composable holds the governing rule:
//
//   If an operation completes before its indicator's delay elapses,
//   nothing is ever shown.
//
// Four states, per the `spin-delay` reference implementation:
//
//   IDLE    source false, nothing shown
//   DELAY   source true, `delay` armed, still reporting false
//   DISPLAY delay fired, reporting true, `minDuration` armed
//   EXPIRE  source went false but the minimum is unmet — keep reporting
//           true until it elapses, so a 510ms operation doesn't produce a
//           10ms blink
//
// Going false during DELAY returns straight to IDLE: the fast path shows
// nothing at all, which is the entire point.
//
// No framework ships this. SWR, TanStack Query, SvelteKit, htmx and Pinia
// Colada all expose instantaneous booleans; only TanStack *Router* has the
// full API, and its `pendingMs` / `pendingMinMs` naming is what `delay` /
// `minDuration` mirror here.
//
// Nil: `source` accepted as ref, getter or plain boolean — never nil itself.

export interface DelayedPendingOptions {
  /** Time the source must stay truthy before anything is shown. */
  delay?: number
  /** Once shown, the minimum time to keep showing — prevents a blink. */
  minDuration?: number
}

// Defaults suit the explicit-action class (a button label swap under the
// cursor). Callers with a gentler indicator — the navigation bar — pass
// shorter values; see PENDING_TIMINGS.
const DEFAULT_DELAY_MS = 500
const DEFAULT_MIN_DURATION_MS = 400

// A non-finite or negative option would arm a timer that never fires (or
// fires immediately and defeats the gate), so coerce rather than trust.
// 0 is legal and meaningful: it degrades that half of the gate to
// instantaneous without arming a timer at all.
function sanitize(value: number | undefined, fallback: number): number {
  if (value === undefined) return fallback
  if (!Number.isFinite(value) || value < 0) return fallback
  return value
}

/**
 * Gate a pending flag behind a delay and a minimum visible duration.
 *
 * Returns a ref that is true only while an indicator should actually be
 * on screen — never during the delay window, and never for less than
 * `minDuration` once it has appeared.
 */
export function useDelayedPending(
  source: MaybeRefOrGetter<boolean>,
  options: DelayedPendingOptions = {}
) {
  const delay = sanitize(options.delay, DEFAULT_DELAY_MS)
  const minDuration = sanitize(options.minDuration, DEFAULT_MIN_DURATION_MS)

  const visible = ref(false)
  // Timers are tracked separately because they can be armed concurrently:
  // an operation that outlives its delay is in DISPLAY with the minimum
  // ticking while the source may flip at any moment.
  let delayTimer: ReturnType<typeof setTimeout> | null = null
  let minTimer: ReturnType<typeof setTimeout> | null = null
  // Set when the source goes false during DISPLAY but the minimum is unmet
  // (state EXPIRE). The minimum timer reads it to decide whether to hide.
  let hidePending = false

  function clearDelay() {
    if (delayTimer !== null) {
      clearTimeout(delayTimer)
      delayTimer = null
    }
  }

  function clearMin() {
    if (minTimer !== null) {
      clearTimeout(minTimer)
      minTimer = null
    }
  }

  function show() {
    delayTimer = null
    visible.value = true
    hidePending = false
    if (minDuration === 0) return
    minTimer = setTimeout(() => {
      minTimer = null
      // Only hide if the source actually went false while we were holding.
      // If it is still pending, DISPLAY continues until it settles.
      if (hidePending) {
        hidePending = false
        visible.value = false
      }
    }, minDuration)
  }

  watch(
    () => toValue(source),
    (pending) => {
      if (pending) {
        // Re-entering while still visible (a second save before the first
        // indicator cleared) just cancels the pending hide — do not restart
        // the minimum, or a rapid sequence would extend it indefinitely.
        hidePending = false
        if (visible.value) return
        // Already counting down to DISPLAY: leave the existing timer alone.
        // Restarting it here is the classic bug where a flapping source
        // pushes the indicator out forever (cf. topbar.js's delayTimerId
        // guard).
        if (delayTimer !== null) return
        if (delay === 0) {
          show()
          return
        }
        delayTimer = setTimeout(show, delay)
        return
      }

      // Source went false.
      if (delayTimer !== null) {
        // Still in DELAY — the fast path. Nothing was ever shown, and
        // nothing ever will be for this operation.
        clearDelay()
        return
      }
      if (!visible.value) return
      if (minTimer !== null) {
        // DISPLAY with the minimum still running: defer the hide (EXPIRE).
        hidePending = true
        return
      }
      // Minimum already satisfied — hide immediately.
      visible.value = false
    },
    // `sync` so the delay timer is armed (and cancelled) in the same tick
    // the source changes. With the default post-flush timing, a caller that
    // flips the source and settles within the same frame could have the
    // cancellation land after the timer had already been scheduled — the
    // indicator would then flash for exactly the case this gate exists to
    // suppress. Arming a timeout is cheap and side-effect-free, so the
    // usual reason to avoid `sync` (expensive recomputation) does not apply.
    { immediate: true, flush: 'sync' }
  )

  // Timers outlive the component otherwise, and firing after unmount writes
  // to a ref nobody is watching (the leak class RR-YWWAL flagged in
  // useConfirm).
  onScopeDispose(() => {
    clearDelay()
    clearMin()
  })

  return computed(() => visible.value)
}
