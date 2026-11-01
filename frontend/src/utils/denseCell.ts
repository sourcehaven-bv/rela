import type { Component } from 'vue'
import type { EntityType, PropertyDef } from '@/types'
import { defaultRegistry } from '@/widgets/registry'
import {
  densePropertyRoutingHint,
  isDenseEmpty,
  type DenseRoutingHint,
} from '@/widgets/viewRouting'
import { formatCellValue } from './format'

/**
 * A property value resolved for a dense read-only surface: the widget to
 * render and the value shaped the way that widget wants.
 */
export interface DenseCell {
  component: Component
  propertyName: string
  modelValue: unknown
}

/** The widget a dense surface renders a property with, and how it routed. */
export interface DenseEntry {
  component: Component
  hint: DenseRoutingHint
}

/**
 * The widget for a property on a dense read-only surface: a list cell, a
 * dashboard table cell. `propertyDef` may be absent, which routes to text.
 * Resolving walks a Map and can warn, so callers resolve once per column
 * rather than per cell (RR-UD2A).
 */
export function denseEntry(propertyDef: PropertyDef | undefined, propertyName: string): DenseEntry {
  const hint = densePropertyRoutingHint(propertyDef, propertyName)
  return { component: defaultRegistry.resolveDense(hint), hint }
}

/**
 * One cell: the entry's widget with the value shaped the way it wants. A
 * passthrough widget gets `formatted()` (dates, `display: {currency}`
 * amounts, booleans); every other widget formats itself and gets the raw
 * value. Undefined for an empty value: widgets may draw a "no value"
 * placeholder that cells must not show, so the cell stays plain text.
 */
export function denseCell(
  entry: DenseEntry,
  value: unknown,
  formatted: () => string
): DenseCell | undefined {
  if (isDenseEmpty(value)) return undefined
  return {
    component: entry.component,
    propertyName: entry.hint.propertyName,
    modelValue: entry.hint.preformatted ? formatted() : value,
  }
}

/**
 * Resolves property cells for rows of any type, as a list cell does: an enum
 * is a badge with its authored label and style, a currency integer an amount.
 * The entry is kept per (entity type, property), so create one resolver per
 * render pass over a set of rows. An unknown type or property gives
 * undefined, and the caller shows plain text.
 */
export function createDenseCellResolver(): (
  value: unknown,
  propertyName: string,
  entityType: EntityType | undefined,
  tz: string
) => DenseCell | undefined {
  const entries = new Map<EntityType, Map<string, DenseEntry>>()

  return (value, propertyName, entityType, tz) => {
    const propertyDef = entityType?.properties[propertyName]
    if (!entityType || !propertyDef) return undefined
    let perType = entries.get(entityType)
    if (!perType) {
      perType = new Map()
      entries.set(entityType, perType)
    }
    let entry = perType.get(propertyName)
    if (!entry) {
      entry = denseEntry(propertyDef, propertyName)
      perType.set(propertyName, entry)
    }
    return denseCell(entry, value, () => formatCellValue(value, propertyName, entityType, tz))
  }
}
