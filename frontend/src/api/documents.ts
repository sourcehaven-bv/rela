import { api } from './client'
import type { DocumentRenderResponse } from '@/types'

export async function renderDocument(
  docName: string,
  entityId: string,
  refresh = false
): Promise<DocumentRenderResponse> {
  const params = refresh ? { refresh: 'true' } : undefined
  return api.get<DocumentRenderResponse>(`/_documents/${docName}/${entityId}`, params)
}
