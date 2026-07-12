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
    `/_history/${entityType}/${encodeURIComponent(entityId)}`
  )
  return resp.versions
}

/** getVersion returns one version's full (redacted) snapshot. */
export async function getVersion(
  entityType: string,
  entityId: string,
  version: number
): Promise<VersionSnapshot> {
  return api.get<VersionSnapshot>(
    `/_history/${entityType}/${encodeURIComponent(entityId)}/${version}`
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
  version: number
): Promise<RestoreResponse> {
  return api.post<RestoreResponse>(
    `/_history/${entityType}/${encodeURIComponent(entityId)}/${version}/restore`
  )
}

// --- Relation versioning (TKT-92JL8P) ---

/** One row of a relation's version timeline (metadata only). */
export interface RelationVersionMeta {
  version: number
  op: 'create' | 'update' | 'rename' | 'delete'
  from: string
  type: string
  to: string
  created_at: string
  principal: { user: string; tool: string }
  prev_from?: string
  prev_to?: string
  triggered_by?: string
}

/** A full relation version snapshot: metadata plus the relation as it was. */
export interface RelationVersionSnapshot {
  from: string
  type: string
  to: string
  version: number
  op: string
  created_at: string
  principal: { user: string; tool: string }
  relation: {
    from: string
    type: string
    to: string
    content: string
    meta: Record<string, unknown>
  }
}

interface RelationTimelineResponse {
  from: string
  type: string
  to: string
  versions: RelationVersionMeta[]
}

interface RelationRestoreResponse {
  restored_from_version: number
  relation: { from: string; type: string; to: string }
}

function relPath(fromType: string, from: string, relType: string, to: string): string {
  return (
    `/_relation_history/${encodeURIComponent(fromType)}/${encodeURIComponent(from)}` +
    `/${encodeURIComponent(relType)}/${encodeURIComponent(to)}`
  )
}

/**
 * listRelationVersions returns a relation's version timeline (oldest first).
 * fromType is the FROM entity's type (the read gate needs it). PostgreSQL-build
 * capability; other backends respond 501.
 */
export async function listRelationVersions(
  fromType: string,
  from: string,
  relType: string,
  to: string
): Promise<RelationVersionMeta[]> {
  const resp = await api.get<RelationTimelineResponse>(relPath(fromType, from, relType, to))
  return resp.versions
}

/** getRelationVersion returns one relation version's full snapshot. */
export async function getRelationVersion(
  fromType: string,
  from: string,
  relType: string,
  to: string,
  version: number
): Promise<RelationVersionSnapshot> {
  return api.get<RelationVersionSnapshot>(`${relPath(fromType, from, relType, to)}/${version}`)
}

/** restoreRelationVersion restores a relation to a past version (may 409 on a dangling endpoint). */
export async function restoreRelationVersion(
  fromType: string,
  from: string,
  relType: string,
  to: string,
  version: number
): Promise<RelationRestoreResponse> {
  return api.post<RelationRestoreResponse>(
    `${relPath(fromType, from, relType, to)}/${version}/restore`
  )
}
