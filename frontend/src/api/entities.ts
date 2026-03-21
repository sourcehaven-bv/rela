import { api } from './client'
import type { Entity, CreateEntity, ListResponse, ListParams, AnalyzeResult, Template } from '@/types'
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
  targetId: string
): Promise<void> {
  return api.post(`/${getPlural(type)}/${entityId}/relations/${relationName}`, { id: targetId })
}
