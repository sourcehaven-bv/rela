/**
 * Gates a pending flag so a fast operation never flashes an indicator.
 *
 * An indicator wired straight to a boolean spends most of its life appearing
 * for a few frames, because the common response is fast. The rule this holds
 * is: if an operation finishes before the delay elapses, nothing is shown at
 * all. Once something is shown it stays for `minDuration`, so a response that
 * lands just past the delay does not produce a blink.
 *
 * Four states:
 *
 *   idle     nothing shown
 *   delay    source is true, counting down, still reporting false
 *   display  reporting true, minimum armed
 *   expire   source went false before the minimum elapsed, so keep
 *            reporting true until it does
 *
 * `RlActivityBar` holds its own simpler version of this: a navigation bar has
 * no minimum duration, because it is removed by the page changing underneath
 * it rather than by the flag clearing.
 */
import { computed, onScopeDispose, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'

export interface DelayedPendingOptions {
  /** How long the source must stay true before anything is shown. */
  delay?: number
  /** Once shown, the minimum time to keep showing. */
  minDuration?: number
}

/**
 * Timings for the explicit-action indicator, which is a label swap on the
 * control the user just pressed.
 *
 * The delay scales with how invasive the indicator is rather than with how
 * slow the response is. A peripheral bar costs the reader almost nothing and
 * can appear early; a label changing under the cursor is disruptive and waits
 * longer. Published figures around 1000ms are calibrated for a spinner on a
 * button, the most invasive case of all.
 */
export const PENDING_DELAY_MS = 500
export const PENDING_MIN_DURATION_MS = 400

/**
 * A negative or non-finite option would arm a timer that never fires, so it
 * falls back rather than being trusted. Zero is meaningful: it makes that half
 * of the gate immediate without arming a timer.
 */
function sanitize(value: number | undefined, fallback: number): number {
  if (value === undefined) return fallback
  if (!Number.isFinite(value) || value < 0) return fallback
  return value
}

/**
 * Returns a ref that is true only while an indicator should be on screen.
 */
export function useDelayedPending(
  source: MaybeRefOrGetter<boolean>,
  options: DelayedPendingOptions = {},
) {
  const delay = sanitize(options.delay, PENDING_DELAY_MS)
  const minDuration = sanitize(options.minDuration, PENDING_MIN_DURATION_MS)

  const visible = ref(false)
  /*
   * Two pieces of mutable state, meaning different things: `delayTimer` is
   * armed while counting down, and `shownAt` is when the display began or
   * null when nothing is shown. Deriving the remaining hold from a timestamp
   * rather than from a second flag makes "visible, minimum spent, source
   * still pending" unrepresentable; as a flag it let a second operation
   * inherit an expired minimum and vanish the moment it settled.
   */
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
        // A new operation cancels a scheduled hide and takes over whatever is
        // already on screen.
        clearHide()
        if (visible.value) {
          /*
           * Already showing, so this operation adopts the display instead of
           * restarting the delay. Keep `shownAt` while the current period has
           * time left, or a rapid sequence would extend the display forever.
           * Restart it once spent, so this operation gets a real minimum.
           */
          if (shownAt !== null && Date.now() - shownAt >= minDuration) {
            shownAt = Date.now()
          }
          return
        }
        // Already counting down: leave the timer alone. Restarting it is how a
        // flapping source pushes the indicator out indefinitely.
        if (delayTimer !== null) return
        if (delay === 0) {
          show()
          return
        }
        delayTimer = setTimeout(show, delay)
        return
      }

      if (delayTimer !== null) {
        // Still counting down, so nothing was shown and nothing will be.
        clearDelay()
        return
      }
      if (!visible.value || shownAt === null) return

      /*
       * Hold for what is left of the minimum. Deferred through a timer even
       * when the period is spent, so finishing one operation and starting the
       * next in the same breath can cancel the hide instead of hiding and then
       * paying the whole delay again.
       */
      const remaining = Math.max(0, minDuration - (Date.now() - shownAt))
      clearHide()
      hideTimer = setTimeout(hideNow, remaining)
    },
    /*
     * `sync` so every transition is observed. With the default timing a
     * true/false/true sequence inside one tick collapses to a single true,
     * which merges two operations into one display period.
     */
    { immediate: true, flush: 'sync' },
  )

  // Timers outlive the component otherwise, and firing after unmount writes to
  // a ref nobody is watching.
  onScopeDispose(() => {
    clearDelay()
    clearHide()
  })

  return computed(() => visible.value)
}
