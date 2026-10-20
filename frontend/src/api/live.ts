import { api } from './client'
import { getPlural } from './entities'
import { addressSegment } from '@/utils/entityAddress'
import { channelPath, channelsPath, type ChannelAnchor, type ChannelTarget } from './channels'
import type { ChannelState } from '@/types/channels'
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

/**
 * The subscription path for getPresence(type, address). Subscribing
 * it is what lists the user as a viewer (TKT-MHXH87). `address` names the
 * face (`ID@face`) on a type with faces; the server refuses a bare id there.
 */
export function livePresencePath(type: string, address: string): string {
  return livePath(`/${getPlural(type)}/${addressSegment(address)}/_presence`)
}

/**
 * The subscription path for listEntities(type, params), or null when the list
 * has a free-text search. The server refuses `q`: its hits are gated after a
 * capped search over every entity, so a hidden write can change them. One
 * refused path fails the whole subscription set, so such a list stays off it.
 */
export function liveListPath(type: string, params?: ListParams): string | null {
  const { q, ...rest } = params ?? {}
  if (q) return null
  return livePath(`/${getPlural(type)}`, rest)
}

/**
 * The subscription path for getChannelHead(target). A frame on it means the
 * channel moved: a message was posted, edited or removed, a signal arrived,
 * or the lifecycle changed (TKT-91BK6F).
 */
export function liveChannelPath(t: ChannelTarget): string {
  return livePath(channelPath(t))
}

/** The subscription path for listChannels(anchor, state). */
export function liveChannelsPath(a: ChannelAnchor, state?: ChannelState): string {
  return livePath(channelsPath(a), state ? { state } : undefined)
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
