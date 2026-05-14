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

// EntityPatch is the union of legacy IDs-only and modern JSON:API §9
// relation shapes the unified PATCH endpoint accepts. The body must
// not mix shapes (`shape_mixed` 400); the SPA's body-assembly helper
// in DynamicForm ensures all-modern-or-all-legacy.
export type EntityPatch = Omit<Partial<Entity>, 'relations'> & {
  relations?: Record<string, string[]> | ModernRelationsField
}

export async function updateEntity(
  type: string,
  id: string,
  patch: EntityPatch,
  etag?: string
): Promise<Entity> {
  return api.patch<Entity>(`/${getPlural(type)}/${id}`, patch, etag)
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
