import { api } from './client'

export interface ActionResponse {
  redirect?: string
  message?: string
  message_type?: 'success' | 'info' | 'warning' | 'error'
  // Error fields (returned with non-2xx status)
  error?: string
  correlation_id?: string
  // What an `llm:` action proposes. The action wrote nothing; the user
  // applies the changes they accept through the ordinary write APIs.
  proposal?: ActionProposal
  // What a batch run of an `llm:` action proposes (TKT-I2SRNWR). Like
  // `proposal`, nothing was written.
  batch?: ActionBatch
}

export interface ActionProposal {
  entity: string
  /** Display title; set on the proposals of a batch run. */
  title?: string
  set: { property: string; current: unknown; proposed: unknown }[]
  relate: { relation: string; to: string; to_type: string; to_title: string }[]
  model?: string
}

export interface ActionBatch {
  proposals: ActionProposal[]
  /** Entities whose part of the AI reply could not be used. */
  failed: { entity: string; title?: string; reason: string }[]
  /** Addresses sent that were not run: not readable or not eligible. */
  skipped: number
}

/**
 * Run a batch `llm:` action over several entities in one request
 * (TKT-I2SRNWR). The server gates each address and returns one proposal per
 * entity it ran on. Nothing is written.
 */
export async function runBatchAction(id: string, addresses: string[]): Promise<ActionBatch> {
  const resp = await api.post<ActionResponse | null>(`/_action/${id}`, { entity_ids: addresses })
  return resp?.batch ?? { proposals: [], failed: [], skipped: addresses.length }
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
