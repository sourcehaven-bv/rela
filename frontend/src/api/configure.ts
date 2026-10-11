import { api } from './client'
import { ApiError } from './errors'
import type { TreeValue } from '@/configure/tree'

/**
 * The Configure API (TKT-F5NGMG): the data model and the screens as trees,
 * and the calls that check and save an edited copy.
 *
 * Hand-written to match `internal/configedit` (service.go, plan.go). The
 * server answers 404 when it runs without `--config-editing` and 403 when the
 * principal lacks the `config:edit` permission.
 */

/** The two files the Configure space edits, as the server names them. */
export type ConfigFile = 'schema.yaml' | 'data-entry.yaml'

/** `GET /_configure`: the files as trees, with their version. */
export interface ConfigSnapshot {
  version: string
  schema: TreeValue
  data_entry: TreeValue
  /** The key patterns the Configure space may change, per file. */
  editable: Partial<Record<ConfigFile, string[]>>
}

/** A property renamed rather than removed and added, so its values move. */
export interface ConfigRename {
  entity_type: string
  from: string
  to: string
}

/** Where records holding a removed option go. `property` is its name after the save. */
export interface ConfigValueMapping {
  entity_type: string
  property: string
  from: string
  to: string
}

/** An edited configuration. A file left out is unchanged. */
export interface ConfigDraftBody {
  base_version: string
  schema?: TreeValue
  data_entry?: TreeValue
  renames?: ConfigRename[]
  value_mappings?: ConfigValueMapping[]
  migration_title?: string
}

/**
 * Something the draft must resolve before it can be saved.
 *
 * `code` is one of value_in_use, type_change, unsupported_change, rename,
 * acl_reference, invalid, locked, migration_state.
 */
export interface ConfigProblem {
  code: string
  message: string
  file?: ConfigFile
  /** Dot-separated, with list indexes: `forms.edit_task.fields.2`. */
  path?: string
  entity_type?: string
  property?: string
  value?: string
  count?: number
}

/** One migration step: rename_property, map_values or convert. */
export interface ConfigMigrationStep {
  kind: string
  entity_type: string
  property: string
  from?: string
  to?: string
  count: number
}

export interface ConfigMigration {
  file: string
  title: string
  steps: ConfigMigrationStep[]
}

/** The answer to a preview or a save. */
export interface ConfigResult {
  version: string
  problems: ConfigProblem[] | null
  migration?: ConfigMigration
  saved: boolean
  /** The save switched configuration but its migration did not finish. */
  incomplete?: boolean
}

const BASE = '/_configure'

export function getConfigure(): Promise<ConfigSnapshot> {
  return api.get<ConfigSnapshot>(BASE)
}

/** Checks a draft and describes what saving it would do. Writes nothing. */
export function previewConfigure(draft: ConfigDraftBody): Promise<ConfigResult> {
  return api.post<ConfigResult>(`${BASE}/preview`, draft)
}

/**
 * Saves a draft. A draft with problems is refused with 422 and the same
 * result body a preview gives, which is returned here rather than thrown, so
 * the caller shows the problems the same way. A conflict (409) or a bad draft
 * (400) is thrown as an ApiError.
 */
export async function saveConfigure(draft: ConfigDraftBody): Promise<ConfigResult> {
  try {
    return await api.post<ConfigResult>(`${BASE}/save`, draft)
  } catch (err) {
    const body = refusedResult(err)
    if (body) return body
    throw err
  }
}

/** Finishes a migration a save left incomplete. */
export async function finishConfigureMigration(): Promise<void> {
  await api.post<void>(`${BASE}/migrate`)
}

/** The result body of a 422, which carries problems rather than a problem document. */
function refusedResult(err: unknown): ConfigResult | null {
  if (!(err instanceof ApiError) || err.status !== 422) return null
  const original = err.original as { response?: { data?: unknown } } | undefined
  const data = original?.response?.data
  if (data && typeof data === 'object' && 'problems' in data) return data as ConfigResult
  return null
}
