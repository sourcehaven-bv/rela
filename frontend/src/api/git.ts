import { api } from './client'

export interface GitStatus {
  available: boolean
  branch?: string
  local_changes: number
  remote_ahead: number
  syncing: boolean
  conflict: boolean
  conflict_files?: string[]
}

export interface GitSyncResponse {
  success: boolean
  error?: string
  conflict_files?: string[]
}

export async function getGitStatus(): Promise<GitStatus> {
  return api.get<GitStatus>('/_git/status')
}

export async function syncGit(): Promise<GitSyncResponse> {
  return api.post<GitSyncResponse>('/_git/sync')
}
