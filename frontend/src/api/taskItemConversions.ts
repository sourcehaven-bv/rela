import { api } from './client'
import { ApiError } from './errors'
import type { TaskItemConversionOffer, Warning } from '@/types'

export interface TaskItemConvertRequest {
  source_id: string
  source_face?: string
  text: string
  properties?: Record<string, unknown>
}

export interface TaskItemConversionPreview {
  target_type: string
  required: string[]
}

export interface TaskItemConversionResult {
  definition: string
  entity_id: string
  entity_type: string
  face?: string
  // Empty when the caller may not read the new entity's title.
  title: string
  item: 'link' | 'remove'
  // The create's soft conditions, and a failure of a step after the commit.
  // The entity exists either way.
  warnings?: Warning[]
}

export async function previewTaskItemConversion(
  name: string,
  request: TaskItemConvertRequest
): Promise<TaskItemConversionPreview> {
  return api.post<TaskItemConversionPreview>(
    `/_task_item_conversions/${encodeURIComponent(name)}/preflight`,
    request
  )
}

export async function convertTaskItem(
  name: string,
  request: TaskItemConvertRequest
): Promise<TaskItemConversionResult> {
  return api.post<TaskItemConversionResult>(
    `/_task_item_conversions/${encodeURIComponent(name)}`,
    request
  )
}

/**
 * The id of the entity a failed conversion still created, or null. The
 * server answers `conversion_incomplete` when the entity was committed but
 * the relation to the source was not; converting the item again would create
 * a second entity.
 */
export function incompleteConversionEntity(err: unknown): string | null {
  if (!(err instanceof ApiError)) return null
  const problem = err.problem
  if (!problem?.type.endsWith('/errors/conversion_incomplete')) return null
  return typeof problem.entity_id === 'string' && problem.entity_id ? problem.entity_id : null
}

export type { TaskItemConversionOffer }
