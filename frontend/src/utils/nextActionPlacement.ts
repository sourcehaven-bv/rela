/**
 * Which page a next-action suggestion is resolved for.
 *
 * The server applies the operator's placement (`next_action_surfaces:`,
 * `suggestions:` on a view, `visible_on:` on a source) and resolves
 * context sources for the entity the page is about. It needs to know the
 * page, and only the router knows that, so this maps a route to the three
 * values the GET carries.
 *
 * Keyed by route NAME: a `/s/<space>/...` alias keeps the name of the page it
 * aliases, so a space prefix needs no case of its own.
 */
import type { RouteLocationNormalizedLoaded, RouteRecordNameGeneric } from 'vue-router'

/** A surface kind. Mirrors dataentryconfig.NextActionSurface. */
export type NextActionSurface =
  | 'dashboard'
  | 'list'
  | 'kanban'
  | 'calendar'
  | 'gantt'
  | 'entity'
  | 'document'
  | 'page'
  | 'form'
  | 'search'
  | 'other'

export interface NextActionPlacement {
  /** The kind of page; sent as the `surface` query parameter. */
  kind: NextActionSurface
  /** The view id; for the entity page, the entity type. */
  view?: string
  /** The entity the page is about, as its address. */
  entity?: string
}

/** The part of a route the mapping reads. A resolved location may carry a null name. */
type RouteLike = Pick<RouteLocationNormalizedLoaded, 'params'> & {
  name?: RouteRecordNameGeneric | null
}

function param(route: RouteLike, key: string): string | undefined {
  const v = route.params?.[key]
  const s = Array.isArray(v) ? v[0] : v
  return s ? String(s) : undefined
}

/**
 * The placement for a route. A route that is not about the data is `other`,
 * and so is no route at all (a component mounted outside a router).
 */
export function nextActionPlacement(route: RouteLike | undefined): NextActionPlacement {
  if (!route) return { kind: 'other' }
  const p = (key: string) => param(route, key)
  switch (route.name) {
    case 'dashboard':
      return { kind: 'dashboard' }
    case 'named-dashboard':
      return { kind: 'dashboard', view: p('name') }
    case 'list':
      return { kind: 'list', view: p('id') }
    case 'kanban':
      return { kind: 'kanban', view: p('id') }
    case 'calendar':
      return { kind: 'calendar', view: p('id') }
    case 'gantt':
      return { kind: 'gantt', view: p('id') }
    case 'entity':
      return { kind: 'entity', view: p('type'), entity: p('id') }
    case 'form-create':
      return { kind: 'form', view: p('id') }
    case 'form-edit':
      return { kind: 'form', view: p('id'), entity: p('entityId') }
    case 'document':
      return { kind: 'document', view: p('name'), entity: p('entityId') }
    case 'standalone-document':
      return { kind: 'document', view: p('name') }
    case 'page':
      return { kind: 'page', view: p('page') }
    case 'entity-page':
      return { kind: 'page', view: p('page'), entity: p('entity') }
    case 'search':
      return { kind: 'search' }
    default:
      return { kind: 'other' }
  }
}

/** A stable identity for a placement, for latching and change detection. */
export function placementKey(p: NextActionPlacement): string {
  return `${p.kind}\u0000${p.view ?? ''}\u0000${p.entity ?? ''}`
}
