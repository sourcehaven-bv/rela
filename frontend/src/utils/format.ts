/**
 * Value formatting utilities
 */

import type { PropertyDef, EntityType } from '@/types'

/**
 * Format a value based on its type for display
 */
export function formatValue(value: unknown, type?: string): string {
  if (value === null || value === undefined) return '-'

  if (type === 'date' && typeof value === 'string') {
    const date = new Date(value)
    if (isNaN(date.getTime())) return '-'
    return date.toLocaleDateString()
  }

  if (type === 'boolean') {
    return value ? 'Yes' : 'No'
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
  if (value === null || value === undefined) return ''

  if (Array.isArray(value)) {
    return value.join(', ')
  }

  if (property && entityType) {
    const propDef = entityType.properties[property]
    if (propDef?.type === 'date' && typeof value === 'string') {
      const date = new Date(value)
      if (isNaN(date.getTime())) return '-'
      return date.toLocaleDateString()
    }
    if (propDef?.type === 'boolean') {
      return value ? 'Yes' : 'No'
    }
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
 * Check if a property definition represents an enum
 */
export function isEnumPropertyDef(propDef: PropertyDef | undefined): boolean {
  if (!propDef) return false
  return propDef.type === 'enum' || (propDef.values?.length ?? 0) > 0
}
