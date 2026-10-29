/**
 * Field order, labels and widgets for one message type: the composer's form
 * and the property list under a message both come from here, so the two
 * cannot disagree.
 */
import type {
  ChannelMessagePresentation,
  ChannelMessageTypeDef,
  ChannelPropertyDef,
  ChannelPropertyKind,
  ChannelWidget,
} from './types'

/** One property, ready to render. */
export interface ResolvedChannelField {
  property: ChannelPropertyDef
  label: string
  placeholder?: string
  help?: string
  widget: ChannelWidget
  /** Columns on a 12-column grid; undefined is the full width. */
  span?: number
}

const DEFAULT_WIDGET: Record<ChannelPropertyKind, ChannelWidget> = {
  string: 'text',
  markdown: 'textarea',
  integer: 'number',
  boolean: 'checkbox',
  date: 'date',
  datetime: 'datetime',
  enum: 'select',
  ref: 'ref',
}

/**
 * The one widget override that changes anything: a string property shown as
 * a textarea. Every other configured widget is the type's default, which the
 * server checks at load; an unknown one falls back to the default.
 */
function widgetFor(property: ChannelPropertyDef, widget: string | undefined): ChannelWidget {
  if (property.type === 'string' && widget === 'textarea') return 'textarea'
  return DEFAULT_WIDGET[property.type]
}

/**
 * Required first, then by name in code-unit order. The server sorts names by
 * bytes; for the ASCII names realtime.yaml uses the two agree, where
 * `localeCompare` would fold case and follow the user's locale.
 */
function byRequiredThenName(a: ChannelPropertyDef, b: ChannelPropertyDef): number {
  const req = Number(b.required ?? false) - Number(a.required ?? false)
  if (req) return req
  return a.name < b.name ? -1 : a.name > b.name ? 1 : 0
}

/** A span on the 12-column grid, or undefined for the full width. */
function clampSpan(span: number | undefined): number | undefined {
  if (!span) return undefined
  return Math.min(Math.max(Math.round(span), 1), 12)
}

/**
 * The fields of a message type in presentation order.
 *
 * Listed fields come first, in the listed order, without the `hidden` ones.
 * The properties the presentation does not list follow, required first and
 * then by name. A listed property the message type lacks is skipped: the
 * server refuses one at load, so this only happens between two reloads.
 */
export function resolveChannelFields(
  type: ChannelMessageTypeDef,
  presentation?: ChannelMessagePresentation,
): ResolvedChannelField[] {
  const byName = new Map(type.properties.map((p) => [p.name, p]))
  const listed = new Set<string>()
  const out: ResolvedChannelField[] = []
  for (const f of presentation?.fields ?? []) {
    const property = byName.get(f.property)
    if (!property || listed.has(f.property)) continue
    listed.add(f.property)
    if (f.hidden) continue
    out.push({
      property,
      label: f.label || property.label,
      placeholder: f.placeholder,
      help: f.help,
      widget: widgetFor(property, f.widget),
      span: clampSpan(f.span),
    })
  }
  const rest = type.properties.filter((p) => !listed.has(p.name)).sort(byRequiredThenName)
  for (const property of rest) {
    out.push({ property, label: property.label, widget: DEFAULT_WIDGET[property.type] })
  }
  return out
}

/** The length of s in Unicode code points, as the server counts `max`. */
export function codePointLength(s: string): number {
  let n = 0
  for (const _ of s) n++
  return n
}

/** The key a `pending` set uses for one operation on one message. */
export function messagePendingKey(seq: number, op: string): string {
  return `message:${seq}:${op}`
}

/** The key a `pending` set uses for a lifecycle operation. */
export function lifecyclePendingKey(op: string): string {
  return `lifecycle:${op}`
}

/** The key a `pending` set uses for a session decision. */
export function decisionPendingKey(key: string): string {
  return `decision:${key}`
}

/**
 * The pending key that blocks every decision without marking one as in
 * flight, such as while the composer sends.
 */
export const DECISIONS_BLOCKED_KEY = 'decision:*'

/** Whether a `pending` set holds any decision key, the blocking one included. */
export function anyDecisionPending(pending: ReadonlySet<string> | undefined): boolean {
  if (!pending) return false
  for (const k of pending) if (k.startsWith('decision:')) return true
  return false
}
