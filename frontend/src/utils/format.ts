/**
 * Value formatting utilities
 */

import { RRule } from 'rrule'
import type { PropertyDef, EntityType } from '@/types'

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
function parseDate(value: string): Date {
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
 * Format a value based on its type for display
 */
export function formatValue(value: unknown, type?: string): string {
  if (value === null || value === undefined) return '-'
  if (Array.isArray(value) && value.length === 0) return '-'

  if (type === 'date' && typeof value === 'string') {
    return formatDate(value) ?? '-'
  }

  if (type === 'boolean') {
    return value ? 'Yes' : 'No'
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
 * Format a cell value for display in a list/table
 */
export function formatCellValue(
  value: unknown,
  property: string | undefined,
  entityType?: EntityType
): string {
  // Cells render empty for null/undefined (vs '-' in formatValue) so blank
  // table cells stay visually quiet; do not delegate this branch to formatValue.
  if (value === null || value === undefined) return ''

  if (property && entityType) {
    const propDef = entityType.properties[property]
    if (propDef?.type === 'date' && typeof value === 'string') {
      return formatDate(value) ?? ''
    }
    if (propDef?.type === 'boolean') {
      return value ? 'Yes' : 'No'
    }
    if (propDef?.type === 'rrule') {
      const single = Array.isArray(value) ? value[0] : value
      return formatValue(single, 'rrule')
    }
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
