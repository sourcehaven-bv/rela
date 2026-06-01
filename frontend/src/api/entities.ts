import { api } from './client'
import type {
  Entity,
  CreateEntity,
  ListResponse,
  ListParams,
  AnalyzeResult,
  Template,
  RelationEntry,
  ModernRelationsField,
} from '@/types'
import { useSchemaStore } from '@/stores/schema'
import { warnIfMissingActions } from '@/utils/affordancesWarning'

function getPlural(type: string): string {
  const schema = useSchemaStore()
  const entityType = schema.entityTypes.get(type)
  return entityType?.plural ?? type + 's'
}

export async function listEntities(
  type: string,
  params?: ListParams
): Promise<ListResponse<Entity>> {
  const path = `/${getPlural(type)}`
  const res = await api.get<ListResponse<Entity>>(path, params as Record<string, unknown>)
  warnIfMissingActions(res, path)
  return res
}

export async function getEntity(
  type: string,
  id: string,
  params?: { include?: string; fields?: string }
): Promise<Entity> {
  const path = `/${getPlural(type)}/${id}`
  const res = await api.get<Entity>(path, params)
  warnIfMissingActions(res, path)
  return res
}

export async function createEntity(type: string, entity: CreateEntity): Promise<Entity> {
  const path = `/${getPlural(type)}`
  const res = await api.post<Entity>(path, entity)
  warnIfMissingActions(res, path)
  return res
}

// dryRunCreateEntity evaluates field/option/relation affordances and
// soft validation against a candidate WITHOUT persisting (TKT-3I5U).
// The create form calls it on mount and (debounced) as the user types
// to gate inputs and surface warnings before commit. The verdicts are
// ADVISORY — the real createEntity re-authorizes. `signal` lets the
// caller drop a stale in-flight request (RR-ZKL2).
//
// Relations are intentionally NOT sent: a candidate has no real ID so
// edges can't be staged; relation affordances reflect the per-type
// verdict only.
export async function dryRunCreateEntity(
  type: string,
  candidate: Pick<CreateEntity, 'id' | 'prefix' | 'properties' | 'content'>,
  signal?: AbortSignal
): Promise<Entity> {
  const path = `/${getPlural(type)}?dry_run=true`
  return api.post<Entity>(path, candidate, { signal })
}

// EntityPatch is the body shape for the unified PATCH endpoint.
// `relations` uses the JSON:API §9 wrapper exclusively; the legacy
// IDs-only form was removed in chore/drop-legacy-relations-shape.
//
// `properties_unset` (TKT-E6094) lets callers express "user cleared
// this field" distinct from "field was untouched". Autosave uses it
// to delete keys atomically alongside property upserts.
export type EntityPatch = Omit<Partial<Entity>, 'relations'> & {
  properties_unset?: string[]
  relations?: ModernRelationsField
}

export async function updateEntity(
  type: string,
  id: string,
  patch: EntityPatch,
  etag?: string,
  signal?: AbortSignal,
): Promise<Entity> {
  const path = `/${getPlural(type)}/${id}`
  const res = await api.patch<Entity>(path, patch, etag, signal)
  warnIfMissingActions(res, path)
  return res
}

export async function deleteEntity(type: string, id: string): Promise<void> {
  return api.delete(`/${getPlural(type)}/${id}`)
}

/**
 * Searches entities by query text, optionally filtered by type.
 * Pass an AbortSignal to cancel an in-flight request — the command palette
 * uses this to abort superseded searches as the user types.
 */
export async function searchEntities(
  query: string,
  type?: string,
  signal?: AbortSignal
): Promise<ListResponse<Entity>> {
  const params: Record<string, string> = { q: query }
  if (type) {
    params.type = type
  }
  return api.get<ListResponse<Entity>>('/_search', params, signal)
}

export async function analyze(): Promise<AnalyzeResult> {
  return api.get<AnalyzeResult>('/_analyze')
}

export async function getTemplates(entityType: string): Promise<Template[]> {
  return api.get<Template[]>(`/_templates/${entityType}`)
}

export async function createRelation(
  type: string,
  entityId: string,
  relationName: string,
  targetId: string,
  meta?: Record<string, unknown>,
  direction?: string
): Promise<void> {
  const body: { id: string; meta?: Record<string, unknown>; direction?: string } = { id: targetId }
  if (meta && Object.keys(meta).length > 0) {
    body.meta = meta
  }
  if (direction === 'incoming') {
    body.direction = direction
  }
  return api.post(`/${getPlural(type)}/${entityId}/relations/${relationName}`, body)
}

export async function getEntityRelations(
  type: string,
  entityId: string,
  relationName: string,
  direction?: string
): Promise<RelationEntry[]> {
  const params = direction === 'incoming' ? { direction } : undefined
  return api.get<RelationEntry[]>(
    `/${getPlural(type)}/${entityId}/relations/${relationName}`,
    params
  )
}

export async function updateRelationProperties(
  type: string,
  entityId: string,
  relationName: string,
  targetId: string,
  meta: Record<string, unknown>,
  direction?: string
): Promise<void> {
  return api.patch(`/${getPlural(type)}/${entityId}/relations/${relationName}/${targetId}`, {
    meta,
    ...(direction === 'incoming' ? { direction } : {}),
  })
}

export async function deleteRelation(
  type: string,
  entityId: string,
  relationName: string,
  targetId: string,
  direction?: string
): Promise<void> {
  const query = direction === 'incoming' ? `?direction=${direction}` : ''
  return api.delete(`/${getPlural(type)}/${entityId}/relations/${relationName}/${targetId}${query}`)
}

