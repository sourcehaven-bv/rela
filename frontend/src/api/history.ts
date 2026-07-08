import { api } from './client'
import type { Entity } from '@/types'

/** One row of an entity's version timeline (metadata only). */
export interface VersionMeta {
  version: number
  op: 'create' | 'update' | 'rename' | 'delete'
  type: string
  created_at: string
  principal: { user: string; tool: string }
  prev_id?: string
  triggered_by?: string
}

/** A full version snapshot: metadata plus the entity as it was, redacted. */
export interface VersionSnapshot {
  id: string
  version: number
  op: string
  created_at: string
  principal: { user: string; tool: string }
  entity: Entity
}

interface TimelineResponse {
  id: string
  versions: VersionMeta[]
}

interface RestoreResponse {
  restored_from_version: number
  entity: Entity
}

/**
 * listVersions returns an entity's version timeline (oldest first). Content
 * versioning is a PostgreSQL-build capability; on other backends the server
 * responds 501 and this rejects — callers should treat that as "history
 * unavailable" rather than an error to surface loudly.
 */
export async function listVersions(entityType: string, entityId: string): Promise<VersionMeta[]> {
  const resp = await api.get<TimelineResponse>(
    `/_history/${entityType}/${encodeURIComponent(entityId)}`,
  )
  return resp.versions
}

/** getVersion returns one version's full (redacted) snapshot. */
export async function getVersion(
  entityType: string,
  entityId: string,
  version: number,
): Promise<VersionSnapshot> {
  return api.get<VersionSnapshot>(
    `/_history/${entityType}/${encodeURIComponent(entityId)}/${version}`,
  )
}

/**
 * restoreVersion restores the entity to a past version. The server applies it
 * as a field-validated write (it may 403 on a field the user cannot write, or
 * 409 if the entity state changed concurrently).
 */
export async function restoreVersion(
  entityType: string,
  entityId: string,
  version: number,
): Promise<RestoreResponse> {
  return api.post<RestoreResponse>(
    `/_history/${entityType}/${encodeURIComponent(entityId)}/${version}/restore`,
  )
}
