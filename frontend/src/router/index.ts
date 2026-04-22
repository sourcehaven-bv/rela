/* v8 ignore start - router configuration, tested via e2e */
import {
  createRouter,
  createWebHistory,
  isNavigationFailure,
  NavigationFailureType,
  type RouteRecordRaw,
} from 'vue-router'
import { isCancelledFetch } from '@/composables/usePageData'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/dashboard',
  },
  {
    path: '/dashboard',
    name: 'dashboard',
    component: () => import('@/views/DashboardView.vue'),
  },
  {
    path: '/list/:id',
    name: 'list',
    component: () => import('@/views/ListView.vue'),
    props: true,
  },
  {
    path: '/form/:id',
    name: 'form-create',
    component: () => import('@/views/FormView.vue'),
    props: true,
  },
  {
    path: '/form/:id/:entityId',
    name: 'form-edit',
    component: () => import('@/views/FormView.vue'),
    props: true,
  },
  {
    path: '/entity/:type/:id',
    name: 'entity',
    component: () => import('@/views/EntityView.vue'),
    props: true,
  },
  {
    path: '/view/:id/:entityId',
    name: 'view',
    component: () => import('@/views/CustomView.vue'),
    props: true,
  },
  {
    path: '/kanban/:id',
    name: 'kanban',
    component: () => import('@/views/KanbanView.vue'),
    props: true,
  },
  {
    path: '/search',
    name: 'search',
    component: () => import('@/views/SearchView.vue'),
  },
  {
    path: '/analyze',
    name: 'analyze',
    component: () => import('@/views/AnalyzeView.vue'),
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('@/views/SettingsView.vue'),
  },
  {
    path: '/conflicts',
    name: 'conflicts',
    component: () => import('@/views/ConflictsView.vue'),
  },
  {
    path: '/document/:name/:entityId',
    name: 'document',
    component: () => import('@/views/DocumentView.vue'),
    props: true,
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior(to, _from, savedPosition) {
    // Browser back/forward: restore the previous scroll position.
    if (savedPosition) return savedPosition
    // Navigation with a hash: scroll the targeted element into view.
    // Targets inside rendered-document panels don't exist at route-change
    // time (the HTML is fetched async after mount), so poll briefly
    // until the element appears — up to ~2s. If the user navigates away
    // or scrolls manually during the wait, bail out without stomping
    // their position.
    if (to.hash) {
      const startPath = to.fullPath
      const startScrollY = window.scrollY
      return waitForElement(to.hash, 2000, () =>
        router.currentRoute.value.fullPath !== startPath || window.scrollY !== startScrollY,
      ).then((found) => {
        if (found) return { el: to.hash, behavior: 'smooth' as const }
        // Element never appeared (or user took over): don't stomp. Use
        // the current scroll position so vue-router doesn't snap to top.
        return { left: window.scrollX, top: window.scrollY }
      })
    }
    // Otherwise: top of the page.
    return { top: 0 }
  },
})

function waitForElement(
  hash: string,
  timeoutMs: number,
  abort: () => boolean,
): Promise<boolean> {
  const id = decodeURIComponent(hash.slice(1)) // strip leading "#" and decode
  return new Promise((resolve) => {
    const deadline = Date.now() + timeoutMs
    const tick = () => {
      if (document.getElementById(id)) return resolve(true)
      if (abort()) return resolve(false)
      if (Date.now() > deadline) return resolve(false)
      requestAnimationFrame(tick)
    }
    tick()
  })
}

// Global navigation error handler.
//
// Without this, vue-router's internal `triggerError` falls back to
// `console.error(error)` for any error thrown during navigation. The
// classes of error we silence are all expected, transient, or impossible
// to act on:
//
//   1. NavigationFailure (aborted / cancelled / duplicated): normal
//      return values from router.push, treated as "errors" only when
//      they're explicitly thrown.
//   2. Cancelled axios fetches that reach the router via a lifecycle
//      hook in a destination component (the BUG-6C3V Firefox race).
//   3. Vite chunk-loader rejections during rapid navigation in Firefox.
//      The dynamic `import('@/views/X.vue')` in routes returns a fetch
//      that Firefox can abort mid-flight when the user navigates away,
//      and Vite's loader sometimes rejects with `undefined` instead of
//      a real Error in that race. We can't fix Vite from here, and the
//      navigation is going to be superseded by the next one anyway, so
//      silencing it is the correct user-facing behaviour.
//
// Anything else is a real navigation error and gets logged with context.
//
// See https://router.vuejs.org/api/#onerror-2 and
// https://router.vuejs.org/guide/advanced/navigation-failures.html.
router.onError((err, to, from) => {
  if (
    isNavigationFailure(err, NavigationFailureType.cancelled) ||
    isNavigationFailure(err, NavigationFailureType.aborted) ||
    isNavigationFailure(err, NavigationFailureType.duplicated)
  ) {
    return
  }
  if (isCancelledFetch(err)) return
  if (err === undefined || err === null) return
  const msg = (err as Error)?.message ?? ''
  // Hard recovery: a chunk loaded but its module state is broken
  // (default export missing, partial module). This happens when a
  // dynamic import races with navigation or a deployment swap. The
  // only recovery is a full reload to refetch the current assets.
  // Rate-limited to once per 10s via sessionStorage — without this
  // a persistently-broken chunk would trigger an infinite reload
  // loop. See https://vite.dev/guide/build.html#load-error-handling.
  if (
    msg.includes("Couldn't resolve component") ||
    msg.includes('Failed to fetch dynamically imported module') ||
    msg.includes('Unable to preload CSS')
  ) {
    try {
      const last = Number(sessionStorage.getItem('rela:lastChunkReload') ?? '0')
      const now = Date.now()
      if (now - last < 10_000) {
        console.warn(
          '[router-error] chunk-load failure, already reloaded recently — skipping:',
          msg,
        )
        return
      }
      sessionStorage.setItem('rela:lastChunkReload', String(now))
    } catch {
      /* sessionStorage unavailable — fall through */
    }
    console.warn('[router-error] chunk-load failure, reloading:', msg)
    window.location.reload()
    return
  }
  // Silent-swallow: known transient races that don't need a reload.
  if (
    msg.includes('Importing a module script failed') ||
    msg.includes('error loading dynamically imported module') ||
    msg === '' // preloadError sometimes surfaces as bare Error with empty message
  ) {
    return
  }
  console.error(
    `[router-error] navigating to=${to.fullPath} from=${from.fullPath}:`,
    err,
  )
})

export default router
/* v8 ignore stop */
