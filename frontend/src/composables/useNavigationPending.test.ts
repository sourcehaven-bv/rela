import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import { useNavigationPending } from './useNavigationPending'

// Driven against a REAL router rather than a stub: the whole point of
// RR-B7U3I8 is which vue-router hooks fire (and don't) for which
// navigation outcomes, so a hand-rolled fake would just re-assert my own
// assumptions about that.

const Blank = defineComponent({ render: () => h('div') })

function makeRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'home', component: Blank },
      { path: '/a', name: 'a', component: Blank },
      { path: '/b', name: 'b', component: Blank },
      { path: '/guarded', name: 'guarded', component: Blank },
      {
        path: '/boom',
        name: 'boom',
        // A route whose component fails to load — the chunk-load failure
        // class that reaches onError rather than afterEach.
        component: () => Promise.reject(new Error('Failed to fetch dynamically imported module')),
      },
    ],
  })
}

describe('useNavigationPending', () => {
  let router: Router
  let nav: ReturnType<typeof useNavigationPending>

  beforeEach(async () => {
    router = makeRouter()
    // Swallow errors so a deliberately-failing navigation doesn't reject
    // the test; the composable's own handler still runs.
    router.onError(() => {})
    await router.push('/')
    await router.isReady()
    nav = useNavigationPending(router)
  })

  afterEach(() => nav.stop())

  it('is idle before anything happens', () => {
    expect(nav.isNavigating.value).toBe(false)
  })

  it('reports navigating between start and settle', async () => {
    let seenMidFlight = false
    const stopProbe = router.beforeEach(() => {
      // Registered after the composable's guard, so it observes the state
      // the composable has already set for this navigation.
      seenMidFlight = nav.isNavigating.value
    })

    await router.push('/a')
    stopProbe()

    expect(seenMidFlight).toBe(true)
    expect(nav.isNavigating.value).toBe(false)
  })

  // THE REGRESSION TEST for RR-B7U3I8. A counter design passes every other
  // test in this file and fails this one: the cancelled navigation
  // increments and never decrements, so the bar stays lit forever.
  it('does not strand the indicator when a navigation is cancelled by a guard', async () => {
    const stopGuard = router.beforeEach((to) => {
      // Redirect away from /guarded — the original navigation is cancelled,
      // so its afterEach never fires for that target.
      if (to.path === '/guarded') return '/b'
      return true
    })

    await router.push('/guarded')
    stopGuard()

    expect(router.currentRoute.value.path).toBe('/b')
    expect(nav.isNavigating.value).toBe(false)
  })

  it('does not strand the indicator when a navigation is aborted', async () => {
    const stopGuard = router.beforeEach((to) => (to.path === '/a' ? false : true))

    await router.push('/a').catch(() => {})
    stopGuard()

    // Aborted: we never arrived, and the indicator must not be left on.
    expect(router.currentRoute.value.path).toBe('/')
    expect(nav.isNavigating.value).toBe(false)
  })

  it('does not strand the indicator when a duplicated navigation is rejected', async () => {
    await router.push('/a')
    // Navigating to the current location is a `duplicated` failure.
    await router.push('/a').catch(() => {})

    expect(nav.isNavigating.value).toBe(false)
  })

  it('does not strand the indicator when the route component fails to load', async () => {
    await router.push('/boom').catch(() => {})

    // onError fired rather than afterEach — the composable's own
    // unconditional handler must still have cleared.
    expect(nav.isNavigating.value).toBe(false)
  })

  // Overlapping navigations supersede rather than accumulate. A counter
  // would need both to settle; identity-tracking clears on the last one.
  it('clears once the superseding navigation settles', async () => {
    const first = router.push('/a')
    const second = router.push('/b')
    await Promise.allSettled([first, second])

    expect(router.currentRoute.value.path).toBe('/b')
    expect(nav.isNavigating.value).toBe(false)
  })

  it('collapses a redirect chain into a single pending window', async () => {
    const stopGuard = router.beforeEach((to) => (to.path === '/guarded' ? '/a' : true))

    await router.push('/guarded')
    stopGuard()

    expect(router.currentRoute.value.path).toBe('/a')
    expect(nav.isNavigating.value).toBe(false)
  })

  it('stop() detaches the guards', async () => {
    nav.stop()
    let sawPending = false
    const stopProbe = router.beforeEach(() => {
      sawPending = nav.isNavigating.value
    })

    await router.push('/a')
    stopProbe()

    expect(sawPending).toBe(false)
    expect(nav.isNavigating.value).toBe(false)
  })

  it('rejects a missing router rather than silently no-op-ing', () => {
    // @ts-expect-error deliberately passing nothing
    expect(() => useNavigationPending(undefined)).toThrow()
  })
})
