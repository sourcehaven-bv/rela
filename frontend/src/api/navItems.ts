import { api } from './client'

/** One entry of a generated nav group: a row of its list, titled for this principal. */
export interface NavItemEntry {
  id: string
  type: string
  label: string
  /** The letter badge. Absent when there is none to show. */
  initial?: string
}

/** The entries of one generated group, in the list's order. */
export interface NavItemList {
  entries: NavItemEntry[]
  /** True when the list holds more rows than the group shows. */
  truncated?: boolean
}

/** `GET /_nav_items`: the entries of each group that declares `items_from:`. */
export interface NavItemsResponse {
  /** Keyed by a sidebar group's `items_key`. A group that could not be read is absent. */
  items: Record<string, NavItemList>
}

/**
 * Fetches the per-principal entries of the sidebar's generated groups.
 *
 * Separate from `/_sidebar` because an entry is an entity title, which is
 * data: the sidebar's structure is served identically to every principal,
 * and these depend on who is asking. With spaces, only the given space's
 * groups are served.
 */
export async function getNavItems(space?: string): Promise<NavItemsResponse> {
  return api.get<NavItemsResponse>('/_nav_items', space ? { space } : undefined)
}
