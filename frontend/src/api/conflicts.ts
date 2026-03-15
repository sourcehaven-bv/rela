import { api } from './client'

export interface ConflictItem {
  path: string
  entity_type?: string
  entity_id?: string
  marker_count: number
}

export interface ConflictsResponse {
  conflicts: ConflictItem[]
  count: number
}

export interface PropertyDiff {
  property: string
  ours_value: string
  theirs_value: string
  is_same: boolean
}

export interface ConflictDetail {
  path: string
  entity_type?: string
  entity_id?: string
  property_diffs: PropertyDiff[]
  content_same: boolean
  content_ours?: string
  content_theirs?: string
}

export interface ResolveRequest {
  path: string
  property_choices: Record<string, 'ours' | 'theirs'>
  content_choice: 'ours' | 'theirs' | 'manual'
  manual_content?: string
}

export async function getConflicts(): Promise<ConflictsResponse> {
  return api.get<ConflictsResponse>('/_conflicts')
}

export async function getConflictDetail(path: string): Promise<ConflictDetail> {
  return api.get<ConflictDetail>(`/_conflicts/${encodeURIComponent(path)}`)
}

export async function resolveConflict(req: ResolveRequest): Promise<{ success: boolean; path: string }> {
  return api.post<{ success: boolean; path: string }>('/_conflicts/resolve', req)
}
