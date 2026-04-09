import { api } from './client'

export interface ActionResponse {
  redirect?: string
  message?: string
  message_type?: 'success' | 'info' | 'warning' | 'error'
  // Error fields (returned with non-2xx status)
  error?: string
  correlation_id?: string
}

/**
 * Run a server-side action by ID. Actions are configured in data-entry.yaml
 * and execute Lua scripts. Returns the script's response (redirect URL or
 * toast message). May return null for 204 No Content responses.
 *
 * When entityId and entityType are provided, the script receives entity
 * context (used by list actions applied to selected rows).
 */
export async function runAction(
  id: string,
  entityId?: string,
  entityType?: string,
): Promise<ActionResponse | null> {
  const body = entityId ? { entity_id: entityId, entity_type: entityType } : undefined
  return api.post<ActionResponse | null>(`/_action/${id}`, body)
}
