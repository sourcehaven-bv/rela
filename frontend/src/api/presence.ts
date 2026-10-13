import { api } from './client'
import { getPlural } from './entities'
import type { PresenceResponse } from '@/types/entity'

/**
 * `GET /{plural}/{address}/_presence` (TKT-MHXH87): who else has this face
 * open. A plain GET never adds the caller; the live subscription on the same
 * path does (see livePresencePath).
 */
export async function getPresence(
  type: string,
  address: string,
  world?: string
): Promise<PresenceResponse> {
  return api.get<PresenceResponse>(
    `/${getPlural(type)}/${address}/_presence`,
    world ? { world } : undefined
  )
}
