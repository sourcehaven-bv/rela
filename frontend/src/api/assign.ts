import { api } from './client'

/**
 * `assign:` blocks (TKT-FGE5G3I): what an item gets when it enters a list or
 * is dropped on a generated sidebar entry. The server resolves the values and
 * writes them through the entity PATCH path, so every write is authorized as
 * a normal edit.
 */

/** Names one block: a list, or a page plus the entry's entity id. */
export type AssignSource = { list: string } | { page: string; target: string }

interface AssignPeer {
  type: string
  id: string
}

/** The resolved block: a create form's prefill, and the PATCH of each row. */
export interface AssignValues {
  entity_type: string
  properties?: Record<string, unknown>
  relations?: Record<string, { data?: AssignPeer[]; add?: AssignPeer[] }>
}

export interface AssignResult {
  id: string
  ok: boolean
  status?: number
  code?: string
  /** For a list: whether the entity is in the list after the write. */
  matches?: boolean
}

export async function getAssign(src: AssignSource, signal?: AbortSignal): Promise<AssignValues> {
  return api.get<AssignValues>('/_assign', { ...src }, signal)
}

export async function postAssign(src: AssignSource, entities: string[]): Promise<AssignResult[]> {
  const res = await api.post<{ results: AssignResult[] }>('/_assign', { ...src, entities })
  return res.results
}
