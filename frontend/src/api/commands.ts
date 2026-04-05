import { api } from './client'
import type { Command } from '@/types'

interface GetCommandsParams {
  pageType: 'entity' | 'list' | 'view' | 'dashboard'
  qualifier?: string
  entityType?: string
}

export async function getCommands(params: GetCommandsParams): Promise<Command[]> {
  const queryParams: Record<string, string> = {
    page_type: params.pageType,
  }
  if (params.qualifier) {
    queryParams.qualifier = params.qualifier
  }
  if (params.entityType) {
    queryParams.entity_type = params.entityType
  }
  return api.get<Command[]>('/_commands', queryParams)
}

// Execute a command and return an EventSource for SSE streaming
export function executeCommand(
  commandId: string,
  context: {
    entityId?: string
    listId?: string
    viewId?: string
  }
): EventSource {
  const params = new URLSearchParams()
  if (context.entityId) params.set('entity_id', context.entityId)
  if (context.listId) params.set('list_id', context.listId)
  if (context.viewId) params.set('view_id', context.viewId)

  const url = `/api/command/${commandId}?${params.toString()}`
  return new EventSource(url)
}
