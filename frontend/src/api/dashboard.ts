import { api } from './client'
import type { DashboardResponse } from '@/types'

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
 */
export async function getDashboard(): Promise<DashboardResponse> {
  return api.get<DashboardResponse>('/_dashboard')
}
