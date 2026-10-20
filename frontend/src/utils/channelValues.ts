import type { ChannelPropertySchema, MessagePropInput, MessagePropValue } from '@/types/channels'

/**
 * Converts channel composer values to message props and back (TKT-91BK6F
 * R20). One module, so every property type converts the same way in the
 * composer, the edit prefill and the tests.
 *
 * The rules follow the server's validateProps (internal/realtime):
 * - A date stays a `YYYY-MM-DD` string and never passes through `Date`, so
 *   no time zone can shift it by a day.
 * - A datetime comes from a `datetime-local` input in the browser's zone
 *   and is sent as UTC RFC 3339. A local time that does not exist (the
 *   gap when clocks move forward) is refused rather than shifted. A local
 *   time that occurs twice (when clocks move back) takes the first.
 * - An empty optional field is absent, never `''`, 0 or NaN.
 * - An integer outside the safe range is refused: JSON would round it.
 * - `max` counts Unicode code points, as the server does.
 */

/** A composer field's raw value: input text, or a checkbox state. */
export type FieldValue = string | boolean | undefined

export type Converted =
  { ok: true; value: MessagePropInput | undefined } | { ok: false; error: string }

const DATE = /^(\d{4})-(\d{2})-(\d{2})$/
const LOCAL_DATETIME = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/
const INTEGER = /^[+-]?\d+$/

/** The length of s in Unicode code points, as the server counts `max`. */
export function codePoints(s: string): number {
  let n = 0
  for (const _ of s) n++
  return n
}

function isEmpty(raw: FieldValue): boolean {
  return raw === undefined || (typeof raw === 'string' && raw.trim() === '')
}

function validDate(y: number, m: number, d: number): boolean {
  if (m < 1 || m > 12 || d < 1) return false
  // Day 0 of the next month is the last day of this one; UTC keeps it
  // independent of the browser's zone.
  return d <= new Date(Date.UTC(y, m, 0)).getUTCDate()
}

/** Formats a Date as UTC RFC 3339 without milliseconds. */
function utcRFC3339(d: Date): string {
  return d.toISOString().replace(/\.\d{3}Z$/, 'Z')
}

function localDatetimeToUTC(raw: string): Converted {
  const m = LOCAL_DATETIME.exec(raw)
  if (!m) return { ok: false, error: 'must be a date and time' }
  const [y, mo, d, h, mi, s] = m.slice(1).map((x) => Number(x ?? 0)) as [
    number,
    number,
    number,
    number,
    number,
    number,
  ]
  if (!validDate(y, mo, d) || h > 23 || mi > 59 || s > 59) {
    return { ok: false, error: 'must be a date and time' }
  }
  const local = new Date(y, mo - 1, d, h, mi, s)
  // A time in a forward gap comes back moved; the composer must not post
  // a time other than the one shown.
  if (
    local.getFullYear() !== y ||
    local.getMonth() !== mo - 1 ||
    local.getDate() !== d ||
    local.getHours() !== h ||
    local.getMinutes() !== mi
  ) {
    return { ok: false, error: 'does not exist in your time zone (the clocks change then)' }
  }
  return { ok: true, value: utcRFC3339(local) }
}

/** Converts one composer value to the prop value a request sends. */
export function toPropValue(prop: ChannelPropertySchema, raw: FieldValue): Converted {
  if (prop.type === 'boolean') {
    if (raw === undefined)
      return prop.required ? { ok: false, error: 'is required' } : { ok: true, value: undefined }
    if (typeof raw !== 'boolean') return { ok: false, error: 'must be yes or no' }
    return { ok: true, value: raw }
  }
  if (isEmpty(raw)) {
    return prop.required ? { ok: false, error: 'is required' } : { ok: true, value: undefined }
  }
  if (typeof raw !== 'string') return { ok: false, error: 'must be text' }
  switch (prop.type) {
    case 'integer': {
      const t = raw.trim()
      if (!INTEGER.test(t)) return { ok: false, error: 'must be a whole number' }
      const n = Number(t)
      if (!Number.isSafeInteger(n)) return { ok: false, error: 'is too large' }
      return { ok: true, value: n }
    }
    case 'date': {
      const m = DATE.exec(raw.trim())
      if (!m || !validDate(Number(m[1]), Number(m[2]), Number(m[3]))) {
        return { ok: false, error: 'must be a date like 2026-01-31' }
      }
      return { ok: true, value: raw.trim() }
    }
    case 'datetime':
      return localDatetimeToUTC(raw.trim())
    case 'enum':
      if (!prop.values?.includes(raw))
        return { ok: false, error: 'must be one of the listed values' }
      return { ok: true, value: raw }
    case 'ref':
      return { ok: true, value: raw.trim() }
    case 'string':
    case 'markdown':
      if (prop.max && codePoints(raw) > prop.max) {
        return { ok: false, error: `must be at most ${prop.max} characters` }
      }
      return { ok: true, value: raw }
  }
  return { ok: false, error: 'has an unknown type' }
}

export type ConvertedProps =
  | { ok: true; props: Record<string, MessagePropInput> }
  | { ok: false; errors: Record<string, string> }

/**
 * Converts every composer value of a message type. Absent values are left
 * out of the props; errors are keyed by property name.
 */
export function toProps(
  props: readonly ChannelPropertySchema[],
  values: Record<string, FieldValue>
): ConvertedProps {
  const out: Record<string, MessagePropInput> = {}
  const errors: Record<string, string> = {}
  for (const p of props) {
    const c = toPropValue(p, values[p.name])
    if (!c.ok) errors[p.name] = c.error
    else if (c.value !== undefined) out[p.name] = c.value
  }
  return Object.keys(errors).length ? { ok: false, errors } : { ok: true, props: out }
}

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

/**
 * The composer value for a stored prop, to prefill an edit. A datetime
 * shows in the browser's zone; a ref gives its id. A restricted ref gives
 * undefined: the server offers no edit on such a message, since the
 * composer could not send it back.
 */
export function fromPropValue(
  prop: ChannelPropertySchema,
  value: MessagePropValue | undefined
): FieldValue {
  if (value === undefined || value === null) return prop.type === 'boolean' ? false : undefined
  switch (prop.type) {
    case 'boolean':
      return value === true
    case 'integer':
      return typeof value === 'number' ? String(value) : undefined
    case 'ref':
      return typeof value === 'object' && !value.restricted ? value.id : undefined
    case 'datetime': {
      if (typeof value !== 'string') return undefined
      const d = new Date(value)
      if (Number.isNaN(d.getTime())) return undefined
      const secs = d.getSeconds() ? `:${pad(d.getSeconds())}` : ''
      const day = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
      return `${day}T${pad(d.getHours())}:${pad(d.getMinutes())}${secs}`
    }
    default:
      return typeof value === 'string' ? value : undefined
  }
}
