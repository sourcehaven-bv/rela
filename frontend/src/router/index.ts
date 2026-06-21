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
  {
    path: '/app/:id',
    name: 'app',
    component: () => import('@/views/AppHostView.vue'),
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
    // Rendered-document panels fetch their HTML async and render more
    // content over the first ~second after it lands (mermaid diagrams,
    // v-html + DOMPurify passes). Doing the scroll here via vue-router
    // fires exactly once — if layout shifts after, the target drifts
    // off-screen. Instead: wait for the element to appear, then delegate
    // to a scroll-settle loop that keeps the element in view until its
    // position stabilises (or the user takes over). We return `false` to
    // tell vue-router not to scroll itself.
    if (to.hash) {
      const startPath = to.fullPath
      scrollToAnchorWhenReady(to.hash, () =>
        router.currentRoute.value.fullPath !== startPath,
      )
      return false
    }
    // Otherwise: top of the page.
    return { top: 0 }
  },
})

// scrollToAnchorWhenReady polls for the target id, and once it appears
// keeps re-scrolling to it as the page's rendered content settles —
// document panels render mermaid diagrams and other lazy content for
// seconds after the initial HTML is mounted, which shifts the target
// off-screen if we only scroll once.
//
// Strategy: scroll immediately when the element appears, then observe
// DOM mutations on the document body. Each mutation triggers a re-scroll
// (preserving the target at the top). Stop after TOTAL_TIMEOUT, or when
// `abort()` returns true (user navigated / scrolled manually).
function scrollToAnchorWhenReady(hash: string, abort: () => boolean) {
  const FIND_TIMEOUT_MS = 2000
  const SETTLE_TIMEOUT_MS = 5000

  const id = decodeURIComponent(hash.slice(1)) // strip leading "#" and decode
  const findDeadline = Date.now() + FIND_TIMEOUT_MS

  const findTick = () => {
    if (abort()) return
    const el = document.getElementById(id)
    if (el) {
      settle(el)
      return
    }
    if (Date.now() > findDeadline) return
    requestAnimationFrame(findTick)
  }

  const settle = (el: HTMLElement) => {
    // Track user-initiated scrolls so we can bail if they take over.
    // An auto-scroll we cause has a matching scrollY after the call
    // resolves; any other scroll means the user took over.
    let expectedScrollY = 0
    const onUserScroll = () => {
      if (Math.abs(window.scrollY - expectedScrollY) > 2) {
        cleanup()
      }
    }
    const reScroll = () => {
      el.scrollIntoView({ behavior: 'auto', block: 'start' })
      expectedScrollY = window.scrollY
    }

    // Initial jump.
    reScroll()

    // Observe mutations below .document-body (v-html + DOMPurify passes,
    // lazy-rendered tables, image loads). Each mutation triggers a
    // re-scroll. Mermaid rendering emits a dedicated `rela:mermaid-rendered`
    // event once all diagrams for a container have been swapped in — we
    // listen for that directly instead of polling, since mermaid is the
    // main source of post-mount layout shift. Stop after SETTLE_TIMEOUT_MS
    // or on user action.
    const container = el.closest('.document-body') || document.body
    const mo = new MutationObserver(reScroll)
    mo.observe(container, { childList: true, subtree: true, characterData: true })

    container.addEventListener('rela:mermaid-rendered', reScroll)
    const deadline = window.setTimeout(cleanup, SETTLE_TIMEOUT_MS)
    const abortTick = window.setInterval(() => {
      if (abort()) cleanup()
    }, 200)
    window.addEventListener('wheel', onUserScroll, { passive: true })
    window.addEventListener('touchmove', onUserScroll, { passive: true })
    window.addEventListener('keydown', onUserScroll)

    function cleanup() {
      mo.disconnect()
      container.removeEventListener('rela:mermaid-rendered', reScroll)
      window.clearTimeout(deadline)
      window.clearInterval(abortTick)
      window.removeEventListener('wheel', onUserScroll)
      window.removeEventListener('touchmove', onUserScroll)
      window.removeEventListener('keydown', onUserScroll)
    }
  }

  findTick()
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
