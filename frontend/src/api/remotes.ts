import { api } from './client'

/**
 * Typed client for the remote-sync routes under `/_remotes` (TKT-0FMYPEZ).
 * The shapes mirror `internal/dataentry/remotes_handler.go`.
 *
 * Every route needs the remote:sync permission and answers 403
 * `permission_required` without it. Each entry has passed the caller's row and
 * field gate, and the counts count only those entries. `_actions` on an entry
 * is the same check its POST runs, so the page shows a control only when the
 * key says so and still handles a refusal.
 */

export type RemoteReason =
  'both_changed' | 'refused' | 'uncertain' | 'superseded' | 'local_deleted' | 'remote_deleted'

/** A field's value as the conflict list keeps it: any JSON value. */
export type RemoteValue =
  string | number | boolean | null | RemoteValue[] | { [key: string]: RemoteValue }

export interface RemoteTypeField {
  local: string
  remote: string
  rule: string
}

export interface RemoteType {
  /** `relation` for a relation type; absent for an entity type. */
  kind?: string
  local: string
  source: string
  fields: RemoteTypeField[]
}

/** One remote on `GET /_remotes`. */
export interface RemoteSummary {
  name: string
  url: string
  description?: string
  /** env, file or secret; empty when the remote has no credential. Never its name or value. */
  credential_kind: string
  run_as?: string
  every?: string
  push: boolean
  conflict: string
  on_gone?: string
  send_deletes?: boolean
  max_clock_skew_seconds: number
  superseded_retention_seconds: number
  types: RemoteType[]
  last_run?: string
  next_run?: string
  next_retry?: string
  running: boolean
  failures: number
  /** Linked rows the caller may read. */
  linked: number
  conflicts: { total: number; by_reason: Partial<Record<RemoteReason, number>> }
}

export type RemoteAction = 'resolve_local' | 'resolve_remote' | 'dismiss' | 'link'

/** One entry of a remote's conflict list. */
export interface RemoteEntry {
  /** `relation` for a relation entry; absent for an entity. */
  kind?: string
  /** The local entity id, or the relation key FROM--type--TO. */
  local: string
  type: string
  address?: string
  /** Absent for a row-level entry, such as an uncertain create. */
  field?: string
  remote_field?: string
  /** The local fields a row-level entry tried to send. */
  fields?: string[]
  reason: RemoteReason
  cause?: string
  status?: number
  rule: string
  fallback?: 'time_withheld' | 'time_untrusted' | 'same_time'
  since: string
  seen: string
  remote_value?: RemoteValue
  remote_value_omitted?: boolean
  superseded?: {
    winner: 'local' | 'remote'
    local_value?: RemoteValue
    local_value_omitted?: boolean
    /** false when the overwritten value was not kept, as under superseded_retention 0. */
    lost_value_kept: boolean
    local_updated: string
    remote_updated: string
  }
  _actions: Record<RemoteAction, boolean>
}

export interface RemoteConflictPage {
  data: RemoteEntry[]
  total: number
  page: number
  per_page: number
  counts: Partial<Record<RemoteReason, number>>
}

/** The entry a POST acts on: its local row, and its field or none. */
export interface RemoteEntryKey {
  local: string
  field?: string
}

function remotePath(name: string): string {
  return `/_remotes/${encodeURIComponent(name)}/conflicts`
}

/** GET every remote with its run health and gated counts. */
export async function listRemotes(signal?: AbortSignal): Promise<RemoteSummary[]> {
  const res = await api.get<{ data: RemoteSummary[] }>('/_remotes', undefined, signal)
  return res.data ?? []
}

/** GET one page of a remote's conflict list. */
export async function listRemoteConflicts(
  name: string,
  page: number,
  perPage: number,
  signal?: AbortSignal
): Promise<RemoteConflictPage> {
  return api.get<RemoteConflictPage>(remotePath(name), { page, per_page: perPage }, signal)
}

function entryBody(key: RemoteEntryKey): { local: string; field: string } {
  return { local: key.local, field: key.field ?? '' }
}

/** POST a resolve: keep the local side or take the remote side. */
export async function resolveRemoteEntry(
  name: string,
  key: RemoteEntryKey,
  take: 'local' | 'remote'
): Promise<{ changed: number }> {
  return api.post(`${remotePath(name)}/_resolve`, { ...entryBody(key), take })
}

/** POST a dismiss of a superseded entry. */
export async function dismissRemoteEntry(
  name: string,
  key: RemoteEntryKey
): Promise<{ changed: number }> {
  return api.post(`${remotePath(name)}/_dismiss`, entryBody(key))
}

/** POST a link of an uncertain create to the remote row at address (type/id). */
export async function linkRemoteEntry(
  name: string,
  local: string,
  address: string
): Promise<{ changed: number }> {
  return api.post(`${remotePath(name)}/_link`, { local, address })
}
