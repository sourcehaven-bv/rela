import { api } from './client'

/**
 * What an audit record is about (Go: audit.Subject). An entity record sets
 * `type`, `id` and `face`; a relation record sets `relation_type`, `from_id`,
 * `from_face` and `to_id`.
 */
export interface AuditSubject {
  kind: string
  type?: string
  id?: string
  face?: string
  relation_type?: string
  from_id?: string
  from_face?: string
  to_id?: string
}

/**
 * One gated change of one entity face (Go: visibility.ChangeRecord). The
 * server leaves out records naming an entity the reader cannot read, and cuts
 * the text to what the reader may see.
 */
export interface ChangeRecord {
  seq: number
  time: string
  op: string
  subject?: AuditSubject
  /** Rename only: the subject before and after. */
  before?: AuditSubject
  after?: AuditSubject
  user?: string
  tool?: string
  summary?: string
  trigger?: string
}

export interface ChangesPage {
  changes: ChangeRecord[]
  /** The `before` cursor of the next page; absent on the last page. */
  next_before?: number
  /** The request's world resolves no face, so there is no change list. */
  world_face_absent?: boolean
}

/** A whole stored audit record (Go: audit.Record), served to audit:read only. */
export interface AuditRecord {
  time: string
  op: string
  subject?: AuditSubject
  before?: AuditSubject
  after?: AuditSubject
  principal: { user: string; tool: string }
  triggered_by?: string
  asserted_by?: string
  summary?: string
  flow?: string
  run?: string
  step?: string
  by?: string
  source?: string
}

export interface AuditEntry {
  seq: number
  record: AuditRecord
}

export interface AuditPage {
  records: AuditEntry[]
  next_before?: number
}

function pageParams(opts: { before?: number; limit?: number; world?: string }) {
  const params: Record<string, unknown> = {}
  if (opts.before) params.before = opts.before
  if (opts.limit) params.limit = opts.limit
  if (opts.world) params.world = opts.world
  return params
}

/**
 * getChanges reads one page of an entity face's changes, newest first
 * (TKT-X1ZFO04). `entityId` is the address on screen. A server without a
 * database audit log answers 404.
 */
export async function getChanges(
  entityType: string,
  entityId: string,
  opts: { before?: number; limit?: number; world?: string } = {},
  signal?: AbortSignal
): Promise<ChangesPage> {
  return api.get<ChangesPage>(
    `/_changes/${encodeURIComponent(entityType)}/${encodeURIComponent(entityId)}`,
    pageParams(opts),
    signal
  )
}

/**
 * getAuditLog reads one page of the whole audit log, newest first. It needs
 * the audit:read permission; without it the server answers 403.
 */
export async function getAuditLog(
  opts: { before?: number; limit?: number } = {},
  signal?: AbortSignal
): Promise<AuditPage> {
  return api.get<AuditPage>('/_audit', pageParams(opts), signal)
}

/** An entity address: `ID` or `ID@face`. */
function address(id: string | undefined, face: string | undefined): string {
  if (!id) return ''
  return face ? `${id}@${face}` : id
}

/**
 * describeSubject says what a record is about in one line of plain text.
 * With `self`, the id of the entity on screen, a relation names only its
 * other end.
 */
export function describeSubject(
  r: { subject?: AuditSubject; before?: AuditSubject; after?: AuditSubject },
  self?: string
): string {
  if (r.before && r.after) {
    return `${address(r.before.id, r.before.face)} → ${address(r.after.id, r.after.face)}`
  }
  const s = r.subject
  if (!s) return ''
  if (s.relation_type) {
    const from = address(s.from_id, s.from_face)
    const to = s.to_id ?? ''
    if (self && s.from_id === self) return `${s.relation_type} → ${to}`
    if (self && s.to_id === self) return `${s.relation_type} ← ${from}`
    return `${from} ${s.relation_type} → ${to}`
  }
  const target = address(s.id, s.face)
  return s.type && target ? `${s.type} ${target}` : target || s.type || ''
}
