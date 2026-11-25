/* v8 ignore start - router configuration, tested via e2e */
import {
  createRouter,
  createWebHistory,
  isNavigationFailure,
  NavigationFailureType,
  type RouteRecordRaw,
} from 'vue-router'
import { isCancelledFetch } from '@/composables/usePageData'
import { useNavigationPending } from '@/composables/useNavigationPending'
import { relaBase } from '@/api/base'
import { currentAsOf } from '@/api/asOf'
import { getSidebar } from '@/api/schema'
import {
  currentSpace,
  lastSpace,
  homeTarget,
  spaceOf,
  useSpaceStore,
  withSpace,
} from '@/stores/space'
import { CONFIGURE_PREFIX, configureRoute, isConfigurePath } from '@/configure/routes'
import { prefixResolvedLinks } from './spaceLinks'
import { setLinkSpace } from '@/utils/spacePath'
import { keepInView } from '@/utils/keepInView'

const routes: RouteRecordRaw[] = [
  {
    /*
     * The root opens the home of the current space (TKT-JEAR2UA): the one
     * the user is in, else the one they were last in, else the first they
     * may enter. Without `spaces:` it opens the dashboard, as it always did.
     *
     * A guard rather than a `redirect` for the reason the `/s/:space` route
     * gives: on a cold load the space store is still empty, so this asks the
     * server, which resolves the remembered space against the principal's
     * permitted ones. The answer fills the store, so a later visit to the
     * root needs no request.
     */
    path: '/',
    // Never rendered: the guard always redirects.
    component: { render: () => null },
    beforeEnter: async () => {
      const store = useSpaceStore()
      if (store.loaded) return homeTarget(store.spaces, store.current)
      try {
        const data = await getSidebar(lastSpace())
        store.set(data)
        return homeTarget(store.spaces, store.current)
      } catch {
        return '/dashboard'
      }
    },
  },
  {
    path: '/dashboard',
    name: 'dashboard',
    component: () => import('@/views/DashboardView.vue'),
  },
  {
    // A named dashboard under `dashboards:` (TKT-5LQNLY).
    path: '/dashboard/:name',
    name: 'named-dashboard',
    component: () => import('@/views/DashboardView.vue'),
    props: true,
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
    path: '/history/:type/:id',
    name: 'history',
    component: () => import('@/views/HistoryView.vue'),
    props: true,
  },
  {
    path: '/relation-history/:fromType/:from/:relType/:to',
    name: 'relation-history',
    component: () => import('@/views/RelationHistoryView.vue'),
    props: true,
  },
  {
    path: '/kanban/:id',
    name: 'kanban',
    component: () => import('@/views/KanbanView.vue'),
    props: true,
  },
  {
    path: '/calendar/:id',
    name: 'calendar',
    component: () => import('@/views/CalendarView.vue'),
    props: true,
  },
  {
    path: '/gantt/:id',
    name: 'gantt',
    component: () => import('@/views/GanttView.vue'),
    props: true,
  },
  {
    path: '/sankey/:id',
    name: 'sankey',
    component: () => import('@/views/SankeyView.vue'),
    props: true,
  },
  {
    path: '/quick',
    name: 'quick',
    component: () => import('@/views/QuickEntryView.vue'),
  },
  {
    path: '/search',
    name: 'search',
    component: () => import('@/views/SearchView.vue'),
  },
  {
    path: '/inbox',
    name: 'inbox',
    component: () => import('@/views/InboxView.vue'),
  },
  {
    // Flow runs and tasks (TKT-ZAXRXND).
    path: '/flows/tasks',
    name: 'flow-tasks',
    component: () => import('@/views/FlowTasksView.vue'),
  },
  {
    path: '/flows/runs',
    name: 'flow-runs',
    component: () => import('@/views/FlowRunsView.vue'),
  },
  {
    path: '/flows/runs/:id',
    name: 'flow-run',
    component: () => import('@/views/FlowRunView.vue'),
    props: true,
  },
  {
    path: '/analyze',
    name: 'analyze',
    component: () => import('@/views/AnalyzeView.vue'),
  },
  {
    path: '/audit',
    name: 'audit',
    component: () => import('@/views/AuditView.vue'),
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
    // Remote sync status and conflict lists (TKT-0FMYPEZ).
    path: '/remotes',
    name: 'remotes',
    component: () => import('@/views/RemotesView.vue'),
  },
  {
    path: '/remotes/:name',
    name: 'remote-conflicts',
    component: () => import('@/views/RemoteConflictsView.vue'),
    props: true,
  },
  {
    path: '/document/:name/:entityId',
    name: 'document',
    component: () => import('@/views/DocumentView.vue'),
    props: true,
  },
  {
    // Standalone documents — those configured without an `entity_type:`,
    // whose content is company-wide rather than about one entity. Reached
    // from a `document:` navigation entry. A separate route rather than an
    // optional `:entityId?` param so the two document kinds stay distinct
    // in the router as they are in the config and the API.
    path: '/document/:name',
    name: 'standalone-document',
    component: () => import('@/views/DocumentView.vue'),
    props: true,
  },
  {
    // A page of `pages:` with its tab bar (TKT-ITQ0HL). Without a tab it
    // opens the first tab the principal may see; see PageView.
    path: '/p/:page/:tab?',
    name: 'page',
    component: () => import('@/views/PageView.vue'),
    props: true,
  },
  {
    // A tab of an entity page, which shows one entity (the anchor). The
    // two-segment form `/p/<page>/<entity>` matches the route above; PageView
    // reads its second segment as the entity and opens the first tab.
    path: '/p/:page/:entity/:tab',
    name: 'entity-page',
    component: () => import('@/views/PageView.vue'),
    props: true,
  },
  {
    path: '/app/:id',
    name: 'app',
    component: () => import('@/views/AppHostView.vue'),
    props: true,
  },
  {
    /*
     * The SPA only runs with a session, and rela-server has no sign-in page
     * of its own: the edge in front of it signs the user in. A `/login`
     * reached with a session (a bookmark, the edge's own link) opens the
     * home page instead of an empty pane (BUG-UZGMHIK). The alias loop below
     * adds `/s/<space>/login`, which lands on the home of that space.
     */
    path: '/login',
    redirect: (to) =>
      typeof to.params.space === 'string' ? `/s/${encodeURIComponent(to.params.space)}` : '/',
  },
  {
    // Any other address shows "Page not found" rather than an empty pane
    // (BUG-UZGMHIK). The alias loop adds the `/s/<space>/...` form.
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('@/views/NotFoundView.vue'),
  },
]

/*
 * Every page also answers under a space prefix, `/s/<space>/...` (TKT-GNKR5H).
 *
 * An alias rather than a second set of routes, so a page keeps one route name
 * and one component whichever way it was reached. The space itself is read by
 * the space store; no view needs the param.
 */
for (const record of routes) {
  if (record.path === '/') continue
  const own = record.alias === undefined ? [] : [record.alias].flat()
  record.alias = [...own, ...[record.path, ...own].map((path) => `/s/:space${path}`)]
}
routes.push({
  // A bare space opens its home page.
  path: '/s/:space',
  // Never rendered: the guard always redirects.
  component: { render: () => null },
  /*
   * A guard rather than a `redirect`, because on a cold load the sidebar has
   * not filled the space store yet, and a redirect would send every space to
   * `/dashboard`. The sidebar waits for the router to be ready, so this asks
   * the server itself instead of waiting for the sidebar.
   */
  beforeEnter: async (to) => {
    const id = String(to.params.space)
    const store = useSpaceStore()
    let spaces = store.spaces
    if (!store.enabled) {
      try {
        spaces = (await getSidebar(id)).spaces ?? []
      } catch {
        spaces = []
      }
    }
    return homeTarget(spaces, id)
  },
})

/*
 * The Configure space (TKT-F5NGMG). Added after the space aliases on
 * purpose: it configures every space and sits in none, so it has no
 * `/s/<space>` form, and the guard below leaves its paths alone.
 */
routes.push({
  path: CONFIGURE_PREFIX,
  component: () => import('@/views/configure/ConfigureLayout.vue'),
  children: [
    { path: '', redirect: configureRoute.entityTypes() },
    {
      path: 'entity-types',
      name: 'configure-entity-types',
      component: () => import('@/views/configure/EntityTypesView.vue'),
    },
    {
      path: 'entity-types/:name',
      name: 'configure-entity-type',
      component: () => import('@/views/configure/EntityTypeView.vue'),
      props: true,
    },
    {
      path: 'choice-lists',
      name: 'configure-choice-lists',
      component: () => import('@/views/configure/ChoiceListsView.vue'),
    },
    {
      path: 'choice-lists/:name',
      name: 'configure-choice-list',
      component: () => import('@/views/configure/ChoiceListView.vue'),
      props: true,
    },
    {
      path: 'relations',
      name: 'configure-relations',
      component: () => import('@/views/configure/RelationsView.vue'),
    },
    {
      path: 'relations/:name',
      name: 'configure-relation',
      component: () => import('@/views/configure/RelationView.vue'),
      props: true,
    },
    {
      path: 'rules',
      name: 'configure-rules',
      component: () => import('@/views/configure/RulesView.vue'),
    },
    {
      path: 'rules/:index',
      name: 'configure-rule',
      component: () => import('@/views/configure/RuleView.vue'),
      props: (route) => ({ index: Number(route.params.index) }),
    },
    {
      path: 'automations',
      name: 'configure-automations',
      component: () => import('@/views/configure/AutomationsView.vue'),
    },
    {
      path: 'automations/:name',
      name: 'configure-automation',
      component: () => import('@/views/configure/AutomationView.vue'),
      props: true,
    },
    {
      path: 'navigation',
      name: 'configure-navigation',
      component: () => import('@/views/configure/NavigationView.vue'),
    },
    {
      path: 'forms',
      name: 'configure-forms',
      component: () => import('@/views/configure/FormsView.vue'),
    },
    {
      path: 'forms/:name',
      name: 'configure-form',
      component: () => import('@/views/configure/FormView.vue'),
      props: true,
    },
    {
      path: 'lists',
      name: 'configure-lists',
      component: () => import('@/views/configure/ListsView.vue'),
    },
    {
      path: 'lists/:name',
      name: 'configure-list',
      component: () => import('@/views/configure/ListView.vue'),
      props: true,
    },
    {
      path: 'boards',
      name: 'configure-boards',
      component: () => import('@/views/configure/BoardsView.vue'),
    },
    {
      path: 'boards/:name',
      name: 'configure-board',
      component: () => import('@/views/configure/BoardView.vue'),
      props: true,
    },
    {
      path: 'dashboard',
      name: 'configure-dashboard',
      component: () => import('@/views/configure/DashboardView.vue'),
    },
  ],
})

const router = createRouter({
  history: createWebHistory(relaBase()),
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
      scrollToAnchorWhenReady(to.hash, () => router.currentRoute.value.fullPath !== startPath)
      return false
    }
    // Otherwise: top of the page.
    return { top: 0 }
  },
})

// scrollToAnchorWhenReady polls for the target id, and once it appears
// keeps it in view while the page's rendered content settles: document
// panels render mermaid diagrams and other lazy content for seconds after
// the initial HTML is mounted, which shifts the target off-screen if we only
// scroll once. keepInView re-scrolls whenever the target moves, and stops
// when the page has settled, the user takes over, or `abort()` returns true
// (the user navigated).
function scrollToAnchorWhenReady(hash: string, abort: () => boolean) {
  const FIND_TIMEOUT_MS = 2000
  const SETTLE_TIMEOUT_MS = 5000

  const id = decodeURIComponent(hash.slice(1)) // strip leading "#" and decode
  const findDeadline = Date.now() + FIND_TIMEOUT_MS

  const findTick = () => {
    if (abort()) return
    if (document.getElementById(id)) {
      keepInView(() => document.getElementById(id), { timeoutMs: SETTLE_TIMEOUT_MS, abort })
      return
    }
    if (Date.now() > findDeadline) return
    requestAnimationFrame(findTick)
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
/*
 * Keeps navigation inside the current space.
 *
 * Links throughout the SPA are built as plain paths (`/entity/...`,
 * `/list/...`). Rather than teach each of them about spaces, an unprefixed
 * path is sent to the same path in the current space, so a cross-link or a
 * search result opens where the user already is. With no spaces configured
 * the store is empty and nothing is rewritten.
 *
 * The redirect keeps the navigation's own push or replace. Forcing replace
 * would overwrite the page the user came from, so Back and a form's Cancel
 * would skip it.
 */
router.beforeEach((to) => {
  const space = useSpaceStore()
  if (!space.enabled || !space.current || spaceOf(to.path) || isConfigurePath(to.path)) return
  return { path: withSpace(to.path, space.current), query: to.query, hash: to.hash }
})

// The guard above covers navigation; these give a link's href the same
// space, for router links and for hrefs built outside the router.
prefixResolvedLinks(router, currentSpace)
setLinkSpace(currentSpace)

/**
 * A page reading at a past instant stays there: a link or a navigation
 * without `?as_of=` gets the page's instant (TKT-6KLUT3U). The way back to
 * the present is `useAsOf().setAsOf(undefined)`, which reloads the page.
 */
router.beforeEach((to) => {
  const at = currentAsOf()
  if (!at || to.query.as_of === at) return
  return { path: to.path, query: { ...to.query, as_of: at }, hash: to.hash }
})

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
          msg
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
  console.error(`[router-error] navigating to=${to.fullPath} from=${from.fullPath}:`, err)
})

// Navigation tracking for the global activity bar (TKT-TFSNBY).
//
// Registered here, at the composition root, rather than from a component:
// the guards must exist for the app's whole life, and a component that
// mounts late would miss the first navigation.
//
// The tracker registers its OWN onError handler rather than hooking into
// the one above. Vue Router invokes every registered error handler, and an
// early `return` only exits the handler it appears in — so registration
// ORDER is irrelevant here, and the tracker's unconditional clear runs even
// for the cancelled/aborted/duplicated failures that handler swallows.
// Those are precisely the cases where afterEach never fires, and where a
// beforeEach/afterEach counter would strand the bar on screen permanently
// (RR-B7U3I8, pinned by a mutation-tested regression test).
export const navigationPending = useNavigationPending(router)

export default router
/* v8 ignore stop */
