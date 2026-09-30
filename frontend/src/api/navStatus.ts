import { api } from './client'
import type { NavItemStatus } from 'rela-components/types'

/** `GET /_nav_status`: the status of each nav entry that declares `status:` rules. */
export interface NavStatusResponse {
  /** Keyed by a sidebar item's `status_key`. An entry with nothing to flag is absent. */
  items: Record<string, NavItemStatus>
}

/**
 * Fetches the per-principal counts behind the sidebar's status markers.
 *
 * Separate from `/_config` for the reason `/_dashboard` is: the sidebar's
 * structure is served identically to every principal, and a count is about
 * data, so it depends on who is asking. With spaces, only the given space's
 * entries are counted.
 */
export async function getNavStatus(space?: string): Promise<NavStatusResponse> {
  return api.get<NavStatusResponse>('/_nav_status', space ? { space } : undefined)
}
