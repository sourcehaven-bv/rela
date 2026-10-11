import { api } from './client'
import { getPlural } from './entities'
import type { ListParams } from '@/types/entity'

/**
 * Live views (TKT-HJ5Z9Y). A page subscribes the GETs it shows; the server
 * re-reads each one as the user after a change and sends `live:changed`
 * with the page's key when the response moved.
 *
 * The paths below are built by the same axios instance and parameters the
 * fetchers in entities.ts and views.ts use, so a subscription names the
 * exact request the page made.
 */

export interface LiveSubscription {
  key: string
  path: string
}

// livePath renders url + params the way the API client sends them, relative
// to /api/v1 and without the project base the server strips.
function livePath(url: string, params?: object): string {
  return api.getAxios().getUri({ baseURL: '/api/v1', url, params })
}

/** The subscription path for getEntity(type, id, params). */
export function liveEntityPath(
  type: string,
  id: string,
  params?: { include?: string; fields?: string; world?: string }
): string {
  return livePath(`/${getPlural(type)}/${id}`, params)
}

/** The subscription path for listEntities(type, params). */
export function liveListPath(type: string, params?: ListParams): string {
  return livePath(`/${getPlural(type)}`, params)
}

/** The subscription path for fetchView(type, id, world). */
export function liveViewPath(type: string, id: string, world?: string): string {
  return livePath(`/_views/${type}/${id}`, world ? { world } : undefined)
}

/** Replaces the stream's subscription set. */
export async function putLiveSubscriptions(
  conn: string,
  subscriptions: LiveSubscription[]
): Promise<void> {
  await api.put(`/_events/${encodeURIComponent(conn)}/subscriptions`, { subscriptions })
}
