/**
 * A clock that moves on once a minute, for text that shows elapsed time,
 * such as "Planning · 3h".
 *
 * A `computed` that reads `Date.now()` caches its first answer, because
 * the clock is not reactive. So elapsed time read that way never moves
 * until something else changes. The ref this returns is the reactive
 * dependency that makes it move.
 *
 * Every caller shares one timer. It starts with the first caller and stops
 * when the scope of the last one ends, so a page with no such text runs no
 * timer. Each caller gets its own ref, which the timer sets, so no
 * reactive state lives in the module.
 */
import { getCurrentScope, onScopeDispose, readonly, ref, type Ref } from 'vue'

/** How often the clock moves on. */
export const MINUTE_CLOCK_MS = 60_000

const listeners = new Set<(now: number) => void>()
let timer: ReturnType<typeof setInterval> | undefined

function tick() {
  const now = Date.now()
  for (const listener of listeners) listener(now)
}

/**
 * Returns the time in milliseconds, set again each minute while the
 * calling scope lives. Call it in a component's setup or another effect
 * scope; outside one the subscription is never ended.
 */
export function useMinuteClock(): Readonly<Ref<number>> {
  const now = ref(Date.now())
  const listener = (at: number) => {
    now.value = at
  }
  listeners.add(listener)
  timer ??= setInterval(tick, MINUTE_CLOCK_MS)
  if (getCurrentScope()) {
    onScopeDispose(() => {
      listeners.delete(listener)
      if (listeners.size === 0 && timer !== undefined) {
        clearInterval(timer)
        timer = undefined
      }
    })
  }
  return readonly(now)
}
