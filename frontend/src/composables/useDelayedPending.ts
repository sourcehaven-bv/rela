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
  // Exactly two pieces of mutable state, and they mean different things:
  //   delayTimer — armed while counting down to DISPLAY (state DELAY)
  //   shownAt    — the timestamp DISPLAY began, or null when not visible
  //
  // An earlier version carried a `hidePending` boolean plus a second timer
  // handle. Three flags coordinated across two callbacks admitted a state
  // the four-state model does not name — visible, minimum already elapsed,
  // source still pending — in which a SECOND operation inherited no
  // minimum at all and vanished the instant it settled. Deriving the
  // remaining hold from a timestamp makes that state unrepresentable: if
  // we are visible, `shownAt` says when, and the remaining time follows.
  let delayTimer: ReturnType<typeof setTimeout> | null = null
  let hideTimer: ReturnType<typeof setTimeout> | null = null
  let shownAt: number | null = null

  function clearDelay() {
    if (delayTimer !== null) {
      clearTimeout(delayTimer)
      delayTimer = null
    }
  }

  function clearHide() {
    if (hideTimer !== null) {
      clearTimeout(hideTimer)
      hideTimer = null
    }
  }

  function show() {
    delayTimer = null
    shownAt = Date.now()
    visible.value = true
  }

  function hideNow() {
    clearHide()
    shownAt = null
    visible.value = false
  }

  watch(
    () => toValue(source),
    (pending) => {
      if (pending) {
        // A new operation cancels any scheduled hide: whatever is on
        // screen stays, and this operation now owns it.
        clearHide()
        if (visible.value) {
          // Already displaying, so this operation adopts what is on screen
          // rather than restarting the delay — no blink between the two.
          //
          // If the current period still has time left, keep `shownAt` as
          // is: resetting it on every re-entry would let a rapid sequence
          // extend the display indefinitely. If it is already spent, start
          // a fresh period, so this operation gets a real minimum instead
          // of inheriting an expired one and vanishing the moment it
          // settles.
          if (shownAt !== null && Date.now() - shownAt >= minDuration) {
            shownAt = Date.now()
          }
          return
        }
        // Already counting down: leave the timer alone. Restarting it is
        // the classic bug where a flapping source pushes the indicator out
        // forever (cf. topbar.js's delayTimerId guard).
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
      if (!visible.value || shownAt === null) return

      // DISPLAY or EXPIRE: hold for whatever is left of the minimum.
      //
      // `Math.max(0, …)` rather than an immediate hide even when the
      // period is spent: a settle followed by a new operation in the SAME
      // breath (finish one save, start the next) would otherwise hide on
      // the first transition and re-pay the whole delay on the second — a
      // visible flicker mid-sequence. Deferring through a 0ms timer lets
      // that re-entry cancel the hide first, while still hiding on the
      // next turn of the loop when nothing follows.
      const remaining = Math.max(0, minDuration - (Date.now() - shownAt))
      clearHide()
      hideTimer = setTimeout(hideNow, remaining)
    },
    // `sync` so the machine observes every transition rather than only the
    // last value in a tick. With the default pre-flush timing a
    // true -> false -> true sequence inside one tick collapses to a single
    // `true`, which silently merges two distinct operations into one
    // display period and loses the second one's minimum.
    { immediate: true, flush: 'sync' }
  )

  // Timers outlive the component otherwise, and firing after unmount writes
  // to a ref nobody is watching (the leak class RR-YWWAL flagged in
  // useConfirm).
  onScopeDispose(() => {
    clearDelay()
    clearHide()
  })

  return computed(() => visible.value)
}
