import { api } from './client'
import type { Command } from '@/types'

interface GetCommandsParams {
  pageType: 'entity' | 'list' | 'view' | 'dashboard'
  qualifier?: string
  entityType?: string
}

export async function getCommands(
  params: GetCommandsParams,
  signal?: AbortSignal,
): Promise<Command[]> {
  const queryParams: Record<string, string> = {
    page_type: params.pageType,
  }
  if (params.qualifier) {
    queryParams.qualifier = params.qualifier
  }
  if (params.entityType) {
    queryParams.entity_type = params.entityType
  }
  return api.get<Command[]>('/_commands', queryParams, signal)
}

