import { api } from './client'
import { getPlural } from './entities'
import { addressSegment } from '@/utils/entityAddress'
import type { PresenceResponse } from '@/types/entity'

/**
 * `GET /{plural}/{address}/_presence` (TKT-MHXH87): who else has this face
 * open. A plain GET never adds the caller; the live subscription on the same
 * path does (see livePresencePath). It sends no `?world=`: the server
 * answers a non-default world on `_presence` with 422 world_unsupported
 * (worldCapablePath in internal/dataentry/world.go), and the address already
 * names the face.
 */
export async function getPresence(type: string, address: string): Promise<PresenceResponse> {
  return api.get<PresenceResponse>(`/${getPlural(type)}/${addressSegment(address)}/_presence`)
}
