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
// count, should that ever feed the same bar.
//
// Nil: `router` rejected — the caller must supply a real Router.

export interface NavigationPending {
  /** True while a navigation has started and not yet settled. */
  isNavigating: ComputedRef<boolean>
  /** Detach the guards. Returned mainly so tests don't leak them. */
  stop: () => void
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
    isNavigating: computed(() => pendingPath.value !== null),
    stop: () => {
      removeBefore()
      removeAfter()
      removeError()
      pendingPath.value = null
    },
  }
}
