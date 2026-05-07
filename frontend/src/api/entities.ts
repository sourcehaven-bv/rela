import { api } from './client'
import type { Entity, CreateEntity, ListResponse, ListParams, AnalyzeResult, Template, RelationEntry } from '@/types'
import { useSchemaStore } from '@/stores/schema'

function getPlural(type: string): string {
  const schema = useSchemaStore()
  const entityType = schema.entityTypes.get(type)
  return entityType?.plural ?? type + 's'
}

export async function listEntities(
  type: string,
  params?: ListParams
): Promise<ListResponse<Entity>> {
  return api.get<ListResponse<Entity>>(`/${getPlural(type)}`, params as Record<string, unknown>)
}

export async function getEntity(
  type: string,
  id: string,
  params?: { include?: string; fields?: string }
): Promise<Entity> {
  return api.get<Entity>(`/${getPlural(type)}/${id}`, params)
}

export async function createEntity(type: string, entity: CreateEntity): Promise<Entity> {
  return api.post<Entity>(`/${getPlural(type)}`, entity)
}

export async function updateEntity(
  type: string,
  id: string,
  patch: Partial<Entity>,
  etag?: string
): Promise<Entity> {
  return api.patch<Entity>(`/${getPlural(type)}/${id}`, patch, etag)
}

// JSON:API §5.2.1-shaped resource identifier with rela's per-edge upsert
// semantics on meta and content. See TKT-K2VAA.
export interface ResourceIdentifier {
  type: string
  id: string
  // Meta merges into the existing relation's properties. Absent = leave
  // existing meta untouched. Mirrors entity-level `properties`.
  meta?: Record<string, unknown>
  // Clears the named keys after the merge. Mirrors `properties_unset`.
  meta_unset?: string[]
  // Upserts the relation's markdown body. Absent = leave alone.
  // "" = clear. Only meaningful for relation types declared with
  // `Content: true` in the metamodel.
  content?: string
}

// Wrapper for a single relation type's desired state. JSON:API §9 wire
// shape: replacement at the list level. Absent relation type in the
// outer map = leave alone. data: [] = remove all of that type.
// data: null is equivalent to data: [].
//
// ⚠️ DATA-LOSS FOOTGUN: sending `data: []` deletes EVERY edge of this
// relation type from the entity. If you build PATCH bodies via object
// spread on a not-yet-fetched form state — where the default empty
// form value would naturally be `{ data: [] }` — your first auto-save
// fire silently wipes the entity's edges of that type.
//
// Mitigations:
// - Fetch entity state BEFORE constructing the first auto-save PATCH.
// - If the user hasn't touched the relation type, OMIT it from the
//   request body entirely (absent = leave alone is the safe default).
// - The server rejects `{ "tagged": {} }` (data field absent) with a
//   400 to catch the most common malformed-body case.
//
// See docs/data-entry/api-reference.md for the full contract.
export interface RelationsUpdate {
  data: ResourceIdentifier[]
}

// Full PATCH payload accepted by /api/v1/{plural}/{id}.
export interface UpdateEntityPatch {
  properties?: Record<string, unknown>
  properties_unset?: string[]
  content?: string
  relations?: Record<string, RelationsUpdate>
}

// patchEntity is the unified PATCH for entity properties + content +
// relations. Use this in preference to updateEntity when you need to
// pass the new wire format (relations or properties_unset).
export async function patchEntity(
  type: string,
  id: string,
  patch: UpdateEntityPatch,
  etag?: string
): Promise<Entity> {
  return api.patch<Entity>(`/${getPlural(type)}/${id}`, patch, etag)
}

export async function deleteEntity(type: string, id: string): Promise<void> {
  return api.delete(`/${getPlural(type)}/${id}`)
}

export async function searchEntities(
  query: string,
  type?: string
): Promise<ListResponse<Entity>> {
  const params: Record<string, string> = { q: query }
  if (type) {
    params.type = type
  }
  return api.get<ListResponse<Entity>>('/_search', params)
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

export async function toggleCheckbox(entityId: string, index: number): Promise<string> {
  const formData = new FormData()
  formData.append('entity_id', entityId)
  formData.append('index', String(index))

  const response = await fetch('/api/toggle-checkbox', {
    method: 'POST',
    body: formData,
  })

  if (!response.ok) {
    throw new Error('Failed to toggle checkbox')
  }

  return response.text()
}
