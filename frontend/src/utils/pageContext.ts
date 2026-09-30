import type { LocationQuery, RouteLocationNormalizedLoaded } from 'vue-router'

/**
 * The page tab a link left from, carried as `?from_page=<page>/<tab>` so the
 * destination's Back and Cancel return to the tab (TKT-ITQ0HL). A tab of an
 * entity page carries its entity too: `<page>/<entity>/<tab>`.
 *
 * It sits beside `?from=<list-id>`, which names the list for prev/next
 * navigation and cannot also name the page: one list can be a tab of several
 * pages, and the same list opened on its own has no page at all.
 */
export const FROM_PAGE_KEY = 'from_page'

/** The shape of a page id and a tab id, as the server validates them. */
const SEGMENT = '[a-z][a-z0-9_-]{0,31}'
/** The shape of an entity id, as the server validates it (internal/entity/id.go). */
const ENTITY = '[A-Za-z0-9][A-Za-z0-9_-]{0,127}'
const FROM_PAGE = new RegExp(`^(${SEGMENT})/(?:(${ENTITY})/)?(${SEGMENT})$`)

export interface PageTabRef {
  page: string
  tab: string
  /** The anchor of an entity page. Absent on any other page. */
  entity?: string
}

/** The path of a page tab, without a space prefix. */
export function pageTabPath(ref: PageTabRef): string {
  if (ref.entity) return `/p/${ref.page}/${encodeURIComponent(ref.entity)}/${ref.tab}`
  return `/p/${ref.page}/${ref.tab}`
}

/** The page tab the route shows, or undefined outside a page tab. */
export function currentPageTab(route: RouteLocationNormalizedLoaded): PageTabRef | undefined {
  if (route.name !== 'page' && route.name !== 'entity-page') return undefined
  const { page, tab, entity } = route.params
  if (typeof page !== 'string' || typeof tab !== 'string' || !tab) return undefined
  if (route.name === 'entity-page') {
    return typeof entity === 'string' && entity ? { page, entity, tab } : undefined
  }
  return { page, tab }
}

/**
 * The query a link leaving the route adds so Back returns to the page tab.
 * Empty outside a page tab, so a standalone view's links are unchanged.
 */
export function fromPageQuery(route: RouteLocationNormalizedLoaded): Record<string, string> {
  const ref = currentPageTab(route)
  if (!ref) return {}
  return { [FROM_PAGE_KEY]: ref.entity ? `${ref.page}/${ref.entity}/${ref.tab}` : `${ref.page}/${ref.tab}` }
}

/**
 * The page tab a route's `?from_page=` names. Null when absent or malformed:
 * the value is only ever a page id, a tab id and optionally an entity id,
 * never a path, so it cannot point outside the SPA's page routes.
 */
export function readFromPage(query: LocationQuery): PageTabRef | null {
  const raw = query[FROM_PAGE_KEY]
  if (typeof raw !== 'string') return null
  const m = FROM_PAGE.exec(raw)
  if (!m) return null
  return m[2] ? { page: m[1], entity: m[2], tab: m[3] } : { page: m[1], tab: m[3] }
}
