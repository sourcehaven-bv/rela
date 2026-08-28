import { computed, shallowRef, type ComputedRef } from 'vue'
import type { Router } from 'vue-router'

// Tracks whether a route navigation is in flight, for the global activity
// bar (TKT-TFSNBY).
//
// IDENTITY, NOT A COUNT — this is load-bearing (RR-B7U3I8, critical).
//
// The obvious design is a counter incremented in `beforeEach` and
// decremented in `afterEach`. It leaks. Vue Router runs `afterEach` for
// SUCCESSFUL navigations only, and `router.onError` in `router/index.ts`
// deliberately early-returns for cancelled / aborted / duplicated failures
// and for cancelled fetches — exactly the cases where `afterEach` also
// never fires. BUG-6C3V documents those as routine in Firefox during rapid
// navigation across the app's lazily-imported routes, so a counter would
// drift upward and strand the bar on screen permanently: worse than the
// flashing this whole ticket exists to remove.
//
// Storing the pending target instead makes the leak impossible rather than
// merely unlikely. Each `beforeEach` OVERWRITES the previous value, so a
// superseded navigation cannot leave a residue behind, and any settle
// clears it outright. Redirect chains (several `beforeEach` before one
// `afterEach`) collapse for free.
//
// This deliberately narrows the ticket's "reference-count the indicator"
// edge rule for navigation specifically: navigations supersede one another
// rather than accumulating, so the latest is the only one that matters.
// Reference counting stays correct for a concurrent in-flight *query*
// count, which is exactly what `useRouteLoad` below provides.
//
// ROUTE SETTLE IS NOT ARRIVAL. Measured on the demo: stepping between two
// entities settles the route in ~99ms while the entity's own data lands at
// ~2100ms. The component is already mounted (same route, different param),
// so `afterEach` fires almost immediately and the user then stares at a
// stale page with no indication anything is happening. A bar wired to
// router state alone is therefore correct by its own definition and
// useless in practice.
//
// So a view that fetches its own data reports that fetch through
// `useRouteLoad`, and the bar shows while EITHER the route is in flight or
// the destination is still assembling itself. That is what the ticket's
// "the bar is for NEW content" rule actually meant.
//
// Nil: `router` rejected — the caller must supply a real Router.

export interface NavigationPending {
  /**
   * True while a navigation is in flight OR the destination view is still
   * loading the data it needs to render for the first time.
   */
  isNavigating: ComputedRef<boolean>
  /** Detach the guards. Returned mainly so tests don't leak them. */
  stop: () => void
}

// Count of views currently performing a COLD load — one they have no
// previous content to cover. A count rather than a flag because nested
// regions (an entity page and its documents panel) can load concurrently
// and must not clear each other's contribution; this is the concurrent
// in-flight case where reference counting IS correct.
const routeLoadCount = shallowRef(0)

/**
 * Report a view's cold load to the global activity bar.
 *
 * Call `begin()` when a fetch starts with nothing on screen, and the
 * returned function exactly once when it settles. Views that keep previous
 * content on screen should NOT report — there is nothing for the user to
 * wait for, and the bar would contradict the visible page.
 *
 * Nil: never returns nil; the settle function is idempotent.
 */
export function beginRouteLoad(): () => void {
  routeLoadCount.value += 1
  let settled = false
  return () => {
    // Idempotent: a caller that settles in both a `finally` and an error
    // path must not drive the count negative and pin the bar on.
    if (settled) return
    settled = true
    routeLoadCount.value = Math.max(0, routeLoadCount.value - 1)
  }
}

// Test seam: reset the shared counter between cases.
export function _resetRouteLoadForTest(): void {
  routeLoadCount.value = 0
}

/**
 * Attach navigation-tracking guards to a router.
 *
 * Call once at wiring time. The guards are registered for the life of the
 * app; `stop()` exists for tests.
 */
export function useNavigationPending(router: Router): NavigationPending {
  if (!router) throw new Error('useNavigationPending requires a router')

  // Holds the fullPath of the in-flight navigation, or null when settled.
  // A string rather than the route object so an identical re-navigation
  // can't be mistaken for a different one by reference.
  const pendingPath = shallowRef<string | null>(null)

  const removeBefore = router.beforeEach((to) => {
    pendingPath.value = to.fullPath
    // Return nothing: this guard observes, it never blocks a navigation.
  })

  const removeAfter = router.afterEach(() => {
    pendingPath.value = null
  })

  // Registered as its OWN onError handler rather than folded into the
  // existing one in router/index.ts. That handler's early-returns
  // (cancelled/aborted/duplicated, isCancelledFetch, null error) would skip
  // a clear placed after them, and reordering its branches to accommodate
  // this would put an indicator concern inside three documented
  // Firefox/Vite race workarounds. Vue Router runs every registered
  // handler, so a separate unconditional one cannot be bypassed.
  const removeError = router.onError(() => {
    pendingPath.value = null
  })

  return {
    isNavigating: computed(() => pendingPath.value !== null || routeLoadCount.value > 0),
    stop: () => {
      removeBefore()
      removeAfter()
      removeError()
      pendingPath.value = null
    },
  }
}
