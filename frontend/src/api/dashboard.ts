import { api } from './client'
import type { DashboardCardValue, DashboardResponse } from '@/types'

/**
 * Fetches the dashboard page config with the cards this principal may see.
 *
 * Separate from `/_config` (which carries the `dashboard:` block verbatim for
 * everyone) because this response is per-principal: cards carrying a
 * `permission:` the caller does not hold are omitted server-side (TKT-53KICM).
 *
 * That omission is a UX affordance, not a boundary — each card's query still
 * runs through the ACL-scoped search endpoint, returning exactly what it always
 * did. Do not reintroduce client-side filtering on `card.permission`.
 *
 * `name` selects a dashboard under `dashboards:` (TKT-5LQNLY); without it the
 * default dashboard is served. An unknown name rejects with a 404.
 */
export async function getDashboard(name?: string): Promise<DashboardResponse> {
  return api.get<DashboardResponse>('/_dashboard', name ? { name } : undefined)
}

/**
 * Fetches the server-computed value of one `list:` card (TKT-2SGPJZ).
 *
 * `index` is the card's `index` from {@link getDashboard}. `world` is the
 * page's world, as the card's list would be read in it. Rejects with a 404
 * for an unknown or hidden card.
 */
export async function getDashboardCard(
  name: string | undefined,
  index: number,
  world?: string
): Promise<DashboardCardValue> {
  const params: Record<string, string> = { card: String(index) }
  if (name) params.name = name
  if (world) params.world = world
  return api.get<DashboardCardValue>('/_dashboard/card', params)
}
