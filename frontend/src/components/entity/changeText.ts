// Reader-facing text for one change of the Changes panel (BUG-XR1DUQA). The
// server sends audit ops, ids and configuration names; this turns them into
// the labels and titles the rest of the page shows. The global audit log
// (AuditView) keeps the raw values for operators.
import { t } from '@/i18n'
import type { AuditSubject, ChangeRecord } from '@/api/audit'
import type { EntityType, RelationType } from '@/types'

/** The schema lookups the text needs; the schema store provides them. */
export interface ChangeSchema {
  entityType(name: string): EntityType | undefined
  relationType(name: string): RelationType | undefined
}

// Literal keys, so the unused-key guard can find them.
const opKeys: Record<string, () => string> = {
  'create-entity': () => t('audit.changes.op.created'),
  'update-entity': () => t('audit.changes.op.changed'),
  'delete-entity': () => t('audit.changes.op.deleted'),
  'rename-entity': () => t('audit.changes.op.renamed'),
  'convert-entity': () => t('audit.changes.op.converted'),
  'restore-entity': () => t('audit.changes.op.restored'),
  'purge-deleted-entity': () => t('audit.changes.op.purged'),
  'copy-state': () => t('audit.changes.op.copied'),
  'attach-render': () => t('audit.changes.op.attached'),
  'create-relation': () => t('audit.changes.op.linked'),
  'update-relation': () => t('audit.changes.op.linkChanged'),
  'delete-relation': () => t('audit.changes.op.unlinked'),
}

/** What happened, in words. An op this list does not know shows as sent. */
export function opText(op: string): string {
  return opKeys[op]?.() ?? op
}

/**
 * Who made the change (BUG-9WCZYOX). A user the server named in `titles` (a
 * person's entity id the reader may read) shows by title. A webhook or an
 * integration shows by kind and name: the names are configuration, not data.
 * Any other `system:` principal, and a write with no known user, is "System".
 */
export function actorText(c: ChangeRecord, titles: Record<string, string> = {}): string {
  const user = c.user ?? ''
  if (titles[user]) return titles[user]
  const idpEvent = cut(user, 'webhook:')
  if (idpEvent) return t('audit.changes.actor.idp', { event: idpEvent })
  const hook = cut(user, 'system:webhook:')
  if (hook) return t('audit.changes.actor.webhook', { name: hook })
  const integration = cut(user, 'integration:')
  if (integration) return t('audit.changes.actor.integration', { name: integration })
  if (!user || user === 'unknown' || user.startsWith('system:')) {
    return t('audit.changes.system')
  }
  return user
}

/** The rest of `s` after `prefix`, or '' when it does not start with it. */
function cut(s: string, prefix: string): string {
  return s.startsWith(prefix) ? s.slice(prefix.length) : ''
}

function typeLabel(schema: ChangeSchema, name: string | undefined): string {
  if (!name) return ''
  return schema.entityType(name)?.label || name
}

function entityText(titles: Record<string, string>, id: string | undefined): string {
  if (!id) return ''
  return titles[id] || id
}

/**
 * What the change is about, seen from the entity on screen (`self`, its bare
 * id). A relation names its label and the title of the other end. Seen from
 * its target it takes the inverse label where the schema defines one, as the
 * relation cards do, and otherwise names the other end as the source of the
 * forward label (BUG-9WCZYOX). A conversion names the types; a rename names
 * the ids, since the id is what changed. A change of the entity itself needs
 * no subject.
 */
export function subjectText(
  c: ChangeRecord,
  self: string,
  titles: Record<string, string>,
  schema: ChangeSchema
): string {
  if (c.before && c.after) {
    if (c.op === 'convert-entity') {
      return `${typeLabel(schema, c.before.type)} → ${typeLabel(schema, c.after.type)}`
    }
    return `${c.before.id ?? ''} → ${c.after.id ?? ''}`
  }
  const s: AuditSubject | undefined = c.subject
  if (!s?.relation_type) return ''
  const rel = schema.relationType(s.relation_type)
  const label = rel?.label || s.relation_type
  if (s.from_id === self) return `${label} → ${entityText(titles, s.to_id)}`
  const source = entityText(titles, s.from_id)
  const inverse = rel?.inverse?.label
  if (inverse) return `${inverse} → ${source}`
  return t('audit.changes.incoming', { label, source })
}

/**
 * The fields an update changed, by label: "Changed: Status, Owner". Every
 * other summary is left out; the operation already says what happened.
 */
export function summaryText(c: ChangeRecord, schema: ChangeSchema): string {
  const props = c.summary?.startsWith('updated: ') ? c.summary.slice('updated: '.length) : ''
  if (!props || c.subject?.kind !== 'entity') return ''
  const def = schema.entityType(c.subject.type ?? '')
  const labels = props.split(',').map((p) => def?.properties?.[p]?.label || p)
  return t('audit.changes.changedFields', { fields: labels.join(', ') })
}

/** What made the change, when not a person: an automation or a schedule. */
export function triggerText(c: ChangeRecord): string {
  if (!c.trigger) return ''
  const kind = c.trigger.split(':')[0]
  if (kind === 'automation') return t('audit.changes.by.automation')
  if (kind === 'schedule') return t('audit.changes.by.schedule')
  return t('audit.changes.by.other')
}
