/**
 * Labels and formatting shared by the remotes page and a remote's conflict
 * list (TKT-0FMYPEZ). The maps are literal so the i18n guard sees every key.
 */
import type { StatusColor } from 'rela-components/types'
import type { RemoteEntry, RemoteReason, RemoteSummary, RemoteValue } from '@/api/remotes'
import { ApiError } from '@/api/errors'

type Translate = (key: string, named?: Record<string, unknown>) => string

const REASON_KEYS: Record<RemoteReason, string> = {
  both_changed: 'remotes.reason.both_changed',
  refused: 'remotes.reason.refused',
  uncertain: 'remotes.reason.uncertain',
  superseded: 'remotes.reason.superseded',
  local_deleted: 'remotes.reason.local_deleted',
  remote_deleted: 'remotes.reason.remote_deleted',
}

const CAUSE_KEYS: Record<string, string> = {
  refused: 'remotes.cause.refused',
  edited: 'remotes.cause.edited',
  server_error: 'remotes.cause.server_error',
  no_new_row: 'remotes.cause.no_new_row',
  already_linked: 'remotes.cause.already_linked',
  cancelled: 'remotes.cause.cancelled',
  timeout: 'remotes.cause.timeout',
  no_answer: 'remotes.cause.no_answer',
}

const FALLBACK_KEYS: Record<string, string> = {
  time_withheld: 'remotes.fallback.time_withheld',
  time_untrusted: 'remotes.fallback.time_untrusted',
  same_time: 'remotes.fallback.same_time',
}

/**
 * Whether keeping the local side sends the row again. It does for an
 * uncertain create, which makes a second copy when the first create did reach
 * the remote.
 */
const SENDS_AGAIN: Record<RemoteReason, boolean> = {
  both_changed: false,
  refused: false,
  uncertain: true,
  superseded: false,
  local_deleted: false,
  remote_deleted: false,
}

export function keepLocalSendsAgain(e: RemoteEntry): boolean {
  return SENDS_AGAIN[e.reason] ?? false
}

/** The reasons in display order. */
export const REMOTE_REASONS: RemoteReason[] = [
  'both_changed',
  'refused',
  'uncertain',
  'superseded',
  'local_deleted',
  'remote_deleted',
]

export function reasonLabel(reason: RemoteReason, t: Translate): string {
  return REASON_KEYS[reason] ? t(REASON_KEYS[reason]) : reason
}

/** The cause of a failed push, or the raw code for one this build does not know. */
export function causeLabel(cause: string | undefined, t: Translate): string {
  if (!cause) return ''
  return CAUSE_KEYS[cause] ? t(CAUSE_KEYS[cause]) : cause
}

export function fallbackLabel(fallback: string | undefined, t: Translate): string {
  if (!fallback) return ''
  return FALLBACK_KEYS[fallback] ? t(FALLBACK_KEYS[fallback]) : fallback
}

/** The run state of a remote as a pill. */
export function runState(r: RemoteSummary, t: Translate): { label: string; color: StatusColor } {
  if (r.running) return { label: t('remotes.state.running'), color: 'blue' }
  if (r.failures > 0) return { label: t('remotes.state.failing', { n: r.failures }), color: 'red' }
  if (!r.last_run) return { label: t('remotes.state.neverRun'), color: 'grey' }
  return { label: t('remotes.state.ok'), color: 'green' }
}

const MAX_VALUE_CHARS = 200

/** A JSON value as short display text. */
export function formatValue(value: RemoteValue | undefined): string {
  if (value === undefined || value === null) return ''
  const s = typeof value === 'string' ? value : JSON.stringify(value)
  return s.length > MAX_VALUE_CHARS ? `${s.slice(0, MAX_VALUE_CHARS)}…` : s
}

/** The route of an entity entry's local row; empty for a relation entry. */
export function entryHref(e: RemoteEntry): string {
  if (e.kind === 'relation') return ''
  return `/entity/${encodeURIComponent(e.type)}/${encodeURIComponent(e.local)}`
}

/** A stable key for one entry: its row and its field. */
export function entryKey(e: RemoteEntry): string {
  return `${e.local}\u0000${e.field ?? ''}`
}

/** True when the server refused for lack of the remote:sync permission. */
export function isPermissionRequired(err: unknown): boolean {
  return err instanceof ApiError && err.status === 403 && err.code === 'permission_required'
}
