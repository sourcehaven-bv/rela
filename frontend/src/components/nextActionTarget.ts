import type { RouteLocationNormalizedLoaded, RouteRecordNameGeneric } from 'vue-router'

/** The part of a route {@link leadsHere} compares. */
type RouteTarget = Pick<RouteLocationNormalizedLoaded, 'params' | 'query'> & {
  name?: RouteRecordNameGeneric | null
}

/**
 * Whether `target`, a navigate offer resolved by the router, is the page
 * `here` already shows: the same route, with every parameter and query value
 * the target sets. A space prefix adds a parameter the target lacks, so it
 * still matches. A suggestion that only leads to where the reader is, such
 * as "Open step" with the step form open below it, says nothing new.
 */
export function leadsHere(target: RouteTarget, here: RouteTarget): boolean {
  if (!target.name || target.name !== here.name) return false
  const same = (want: Record<string, unknown>, have: Record<string, unknown>) =>
    Object.entries(want).every(([k, v]) => String(v ?? '') === String(have[k] ?? ''))
  return same(target.params, here.params) && same(target.query, here.query)
}
