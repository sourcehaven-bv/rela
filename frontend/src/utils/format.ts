/**
 * Value formatting utilities
 */

import { RRule } from 'rrule'
import { TZDate } from '@date-fns/tz'
import type { PropertyDef, EntityType } from '@/types'
import { parseExternalRef } from '@/widgets/externalRef'
import { fileRefNames } from './fileRefs'

export const DATE_FORMAT_OPTIONS: Intl.DateTimeFormatOptions = {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
}

const DATE_ONLY_RE = /^(\d{4})-(\d{2})-(\d{2})$/

// Parse a date-only YYYY-MM-DD string in local time so that
// `2024-01-15` renders as Jan 15 in every timezone, not Jan 14
// in zones west of UTC. Other formats (ISO datetime, etc.) fall
// through to the standard Date constructor.
export function parseDate(value: string): Date {
  const m = DATE_ONLY_RE.exec(value)
  if (m) {
    const y = Number(m[1])
    const mo = Number(m[2])
    const d = Number(m[3])
    const date = new Date(y, mo - 1, d)
    // Reject overflow (e.g. 2024-13-45 silently rolls into 2025).
    if (date.getFullYear() !== y || date.getMonth() !== mo - 1 || date.getDate() !== d) {
      return new Date(NaN)
    }
    return date
  }
  return new Date(value)
}

export function formatDate(value: string, locale?: string): string | null {
  const date = parseDate(value)
  if (isNaN(date.getTime())) return null
  return date.toLocaleDateString(locale, DATE_FORMAT_OPTIONS)
}

/**
 * An amount in the reader's locale, as money when `currency` (an ISO 4217
 * code such as EUR) is given: 3500 with EUR reads "€ 3.500" in nl-NL. A
 * whole number shows no decimals; any other shows two.
 *
 * `locale` defaults to the browser's, as formatDate's does. Tests pass one.
 */
export function formatAmount(value: number, currency?: string, locale?: string): string {
  return amountFormatter(Number.isInteger(value) ? 0 : 2, currency, locale).format(value)
}

// Building an Intl.NumberFormat is the expensive part, and a board formats an
// amount per card on every render, so formatters are kept per shape.
const amountFormatters = new Map<string, Intl.NumberFormat>()

function amountFormatter(digits: number, currency?: string, locale?: string): Intl.NumberFormat {
  const key = `${locale ?? ''}|${currency ?? ''}|${digits}`
  let f = amountFormatters.get(key)
  if (!f) {
    f = new Intl.NumberFormat(locale, {
      ...(currency ? { style: 'currency', currency } : {}),
      minimumFractionDigits: digits,
      maximumFractionDigits: digits,
    })
    amountFormatters.set(key, f)
  }
  return f
}

const DECIMAL_RE = /^-?\d+(\.\d+)?$/

/**
 * A stored property value as a number, or undefined when it is not one.
 * Integer properties arrive as JSON numbers. A plain decimal string such as
 * "42" is accepted too, so a value written by hand into a Markdown file still
 * counts; hex, exponent and padded forms are not.
 */
export function numericValue(value: unknown): number | undefined {
  if (typeof value === 'number') return Number.isFinite(value) ? value : undefined
  if (typeof value === 'string' && DECIMAL_RE.test(value)) return Number(value)
  return undefined
}

/**
 * A property value in the property's display format, or undefined when the
 * property has none or the value is not a number. Today the only format is
 * `display: {currency}` on an integer property (TKT-SVLQQ6), so 3500 reads
 * "€ 3.500" for EUR in nl-NL. Every surface that renders a value for reading
 * goes through this; inputs stay plain numbers.
 */
export function formatDisplayNumber(
  value: unknown,
  propDef: PropertyDef | undefined,
  locale?: string
): string | undefined {
  const currency = propDef?.display?.currency
  if (!currency || propDef?.type !== 'integer') return undefined
  const n = numericValue(value)
  return n === undefined ? undefined : formatAmount(n, currency, locale)
}

const DATETIME_FORMAT_OPTIONS: Intl.DateTimeFormatOptions = {
  dateStyle: 'medium',
  timeStyle: 'short',
}

const pad = (n: number): string => String(n).padStart(2, '0')

/**
 * The browser's IANA time zone (e.g. "Europe/Amsterdam"), used as the default
 * display zone when the user has not chosen an override.
 */
export function browserTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone
}

/**
 * Convert a stored UTC RFC3339 instant to the `YYYY-MM-DDTHH:mm` local
 * wall-clock string a native <input type="datetime-local"> expects, expressed
 * in the given IANA time zone. Returns '' for empty/invalid input.
 */
export function utcISOToLocalInput(iso: string | null | undefined, tz: string): string {
  if (!iso) return ''
  const d = new TZDate(iso, tz)
  if (isNaN(d.getTime())) return ''
  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
    `T${pad(d.getHours())}:${pad(d.getMinutes())}`
  )
}

/**
 * Convert a native datetime-local wall-clock string (`YYYY-MM-DDTHH:mm`),
 * interpreted in the given IANA time zone, to a canonical UTC RFC3339 instant
 * (`...Z`). Returns '' for empty/invalid input. Non-integer offsets (e.g.
 * +05:30) are handled by TZDate.
 *
 * DST edge cases: a wall-clock time that does not exist (spring-forward gap)
 * or is ambiguous (fall-back overlap) is NORMALIZED by TZDate to a real
 * instant — so such a local time is not a true round-trip. This is only
 * reachable when a user types a boundary time by hand; the widget's own
 * output (UTC → local → UTC) always round-trips.
 *
 * The seconds group is optional but the datetime-local input is minute-
 * granular, so the widget never supplies seconds (sub-minute precision on a
 * pre-existing value is dropped when the user edits that field).
 */
export function localInputToUtcISO(local: string | null | undefined, tz: string): string {
  if (!local) return ''
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(local)
  if (!m) return ''
  const [, y, mo, d, h, min, s] = m
  const zoned = new TZDate(
    Number(y),
    Number(mo) - 1,
    Number(d),
    Number(h),
    Number(min),
    Number(s ?? 0),
    tz
  )
  if (isNaN(zoned.getTime())) return ''
  return new Date(zoned.getTime()).toISOString().replace(/\.\d{3}Z$/, 'Z')
}

/**
 * Format a stored UTC RFC3339 instant for human display in the given IANA time
 * zone. Deterministic across machines (fixed style + explicit zone). Returns
 * null for un-parseable input so callers can fall back to the raw string.
 */
// A stored value that carries no zone marker (no trailing Z and no ±HH:MM
// offset) is "naive". We interpret a naive value in the display `tz` — the
// same zone the edit widget uses via utcISOToLocalInput — so viewing and
// editing agree (RR-P9NKU7). A value WITH a zone marker is an absolute
// instant and resolves identically however it is parsed.
export const NAIVE_DATETIME_RE = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?$/

export function formatDatetime(value: string, tz: string, locale?: string): string | null {
  // Parse a naive value as wall-clock in `tz`; parse a zoned/absolute value
  // as the instant it denotes. TZDate handles both (a zoned string ignores
  // the tz arg for its instant), keeping display consistent with the input.
  const date = NAIVE_DATETIME_RE.test(value) ? new TZDate(value, tz) : new Date(value)
  if (isNaN(date.getTime())) return null
  try {
    return date.toLocaleString(locale, { ...DATETIME_FORMAT_OPTIONS, timeZone: tz })
  } catch {
    // Defensive: toLocaleString throws RangeError on an invalid tz. The tz
    // is validated upstream today, but degrade gracefully like formatDate.
    return null
  }
}

/**
 * Format a value based on its type for display.
 *
 * `tz` is the display time zone for `datetime` values; callers that honor the
 * user's display-timezone preference pass `uiStore.effectiveTimezone`. It
 * defaults to the browser zone so callers without store access still render a
 * sensible value.
 */
export function formatValue(value: unknown, type?: string, tz: string = browserTimeZone()): string {
  if (value === null || value === undefined) return '-'
  if (Array.isArray(value) && value.length === 0) return '-'

  if (type === 'date' && typeof value === 'string') {
    return formatDate(value) ?? '-'
  }

  if (type === 'datetime' && typeof value === 'string' && value) {
    return formatDatetime(value, tz) ?? value
  }

  if (type === 'boolean') {
    return value ? 'Yes' : 'No'
  }

  if (type === 'external_ref') {
    return parseExternalRef(value)?.id ?? '-'
  }

  if (type === 'rrule' && typeof value === 'string' && value) {
    try {
      // Handle both "FREQ=..." and "DTSTART:... RRULE:FREQ=..." formats
      const rrulePart = value.includes('RRULE:')
        ? value.substring(value.indexOf('RRULE:'))
        : `RRULE:${value}`
      return RRule.fromString(rrulePart).toText()
    } catch {
      return value
    }
  }

  if (Array.isArray(value)) {
    return value.join(', ')
  }

  return String(value)
}

/**
 * Format a cell value for display in a list/table.
 *
 * `tz` is the display time zone for `datetime` cells (pass
 * `uiStore.effectiveTimezone` to honor the user's preference); defaults to the
 * browser zone.
 */
export function formatCellValue(
  value: unknown,
  property: string | undefined,
  entityType?: EntityType,
  tz: string = browserTimeZone()
): string {
  // Cells render empty for null/undefined (vs '-' in formatValue) so blank
  // table cells stay visually quiet; do not delegate this branch to formatValue.
  if (value === null || value === undefined) return ''

  if (property && entityType) {
    const propDef = entityType.properties[property]
    if (propDef?.type === 'date' && typeof value === 'string') {
      return formatDate(value) ?? ''
    }
    if (propDef?.type === 'datetime' && typeof value === 'string') {
      return formatDatetime(value, tz) ?? String(value)
    }
    if (propDef?.type === 'boolean') {
      // A view table cell carries the server's string form, where "false" is
      // truthy; read it as the boolean it spells.
      const b = typeof value === 'string' ? value === 'true' : Boolean(value)
      return b ? 'Yes' : 'No'
    }
    if (propDef?.type === 'rrule') {
      const single = Array.isArray(value) ? value[0] : value
      return formatValue(single, 'rrule')
    }
    if (propDef?.type === 'external_ref') {
      return parseExternalRef(value)?.id ?? ''
    }
    // A file value is a storage path; a cell shows the file's name.
    if (propDef?.type === 'file') {
      return fileRefNames(value).join(', ')
    }
    const amount = formatDisplayNumber(value, propDef)
    if (amount !== undefined) return amount
  }

  if (Array.isArray(value)) {
    return value.join(', ')
  }

  return String(value)
}

/**
 * Get a cell value from an entity
 */
export function getCellValue(
  entity: { id: string; properties: Record<string, unknown>; relations?: Record<string, string[]> },
  column: { property?: string; relation?: string }
): unknown {
  if (column.property) {
    if (column.property === 'id') return entity.id
    return entity.properties[column.property]
  }
  if (column.relation && entity.relations) {
    return entity.relations[column.relation] || []
  }
  return ''
}

/**
 * Check if a property is an enum type (has defined values)
 */
export function isEnumProperty(prop: { type?: string; values?: string[] }): boolean {
  return prop.type === 'enum' || (prop.values?.length ?? 0) > 0
}

/**
 * Coerce a property value to an array of non-empty strings.
 * Used for list-typed properties where the value may be a raw array,
 * a single scalar, or null/undefined.
 */
export function asArray(value: unknown): string[] {
  const items = Array.isArray(value) ? value : value == null || value === '' ? [] : [value]
  return items.map((v) => String(v)).filter((s) => s !== '')
}

/**
 * Check if a property definition represents an enum
 */
export function isEnumPropertyDef(propDef: PropertyDef | undefined): boolean {
  if (!propDef) return false
  return propDef.type === 'enum' || (propDef.values?.length ?? 0) > 0
}
