import { api } from './client'
import type { Entity } from '@/types'

/**
 * Where a version came from, when a MECHANISM produced it rather than a person
 * typing (TKT-VQHPFK). Present only for a mechanism-produced write.
 *
 * The three fields are independently optional for reasons that are not
 * symmetric, so none of them may be defaulted:
 *
 * - The whole object is ABSENT for a direct edit. There is no `kind: 'manual'`
 *   on the wire and there must not be one here: the absence IS the signal, and
 *   the version's `principal` already names who typed it. Rendering a
 *   "manual" marker for every hand edit would make the copy marker meaningless
 *   by marking everything.
 * - `source` may be absent while `kind` and `definition` are present. The
 *   source entity id is gated server-side against this reader's own verdict,
 *   so a source the reader may not know exists is simply not sent. That is a
 *   normal answer, not missing data and not an error — render the rest and
 *   omit the source.
 * - `definition` is operator-authored configuration (the key in the
 *   metamodel's `copies:` map), which is not confidential, so it arrives
 *   ungated.
 */
export interface VersionOrigin {
  /** The mechanism, e.g. 'copy'. Always present when the object is. */
  kind: string
  /** The source coordinate as `ID@face` (`ID` for the default face), when the reader may see it. */
  source?: string
  /** The operator-declared name that produced the write. */
  definition?: string
}

/** One row of an entity's version timeline (metadata only). */
export interface VersionMeta {
  version: number
  op: 'create' | 'update' | 'rename' | 'delete'
  type: string
  created_at: string
  principal: { user: string; tool: string }
  prev_id?: string
  triggered_by?: string
  /**
   * Provenance for a mechanism-produced write. Absent for a direct edit — see
   * [VersionOrigin]. The `op` above stays what it is: a copy genuinely IS a
   * create or an update, and this annotates that rather than replacing it.
   */
  origin?: VersionOrigin
}

/** A full version snapshot: metadata plus the entity as it was, redacted. */
export interface VersionSnapshot {
  id: string
  version: number
  op: string
  created_at: string
  principal: { user: string; tool: string }
  entity: Entity
  /** Provenance, on the same terms as the timeline row's — see [VersionOrigin]. */
  origin?: VersionOrigin
}

interface TimelineResponse {
  id: string
  versions: VersionMeta[]
  /**
   * The FACE this timeline belongs to; '' is the default face.
   *
   * Versioning is per-face, so a timeline that does not name its subject
   * invites the reader to assume the obvious one — which is exactly how a
   * published page came to show the draft's history.
   */
  face?: string
  /**
   * The world resolves NO face for this entity, so there is no history in it.
   * An empty timeline rather than an error: the entity exists and the caller
   * may read it, matching how the entity view answers with `_world_absent`.
   */
  world_face_absent?: boolean
}

/** A timeline plus the face it describes. */
export interface Timeline {
  versions: VersionMeta[]
  face: string
  worldFaceAbsent: boolean
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
export async function listVersions(
  entityType: string,
  entityId: string,
  world?: string
): Promise<Timeline> {
  const resp = await api.get<TimelineResponse>(
    `/_history/${entityType}/${encodeURIComponent(entityId)}`,
    world ? { world } : undefined
  )
  return {
    versions: resp.versions,
    face: resp.face ?? '',
    worldFaceAbsent: resp.world_face_absent === true,
  }
}

/** getVersion returns one version's full (redacted) snapshot. */
export async function getVersion(
  entityType: string,
  entityId: string,
  version: number,
  world?: string
): Promise<VersionSnapshot> {
  return api.get<VersionSnapshot>(
    `/_history/${entityType}/${encodeURIComponent(entityId)}/${version}`,
    world ? { world } : undefined
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
