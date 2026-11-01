import type { Component } from 'vue'
import type { EntityType } from '@/types'
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

/**
 * Resolves property cells the way a list cell does (EntityList): the widget
 * from `densePropertyRoutingHint` and `defaultRegistry.resolveDense`, and for
 * a passthrough widget the value pre-formatted by `formatCellValue` (dates,
 * `display: {currency}` amounts, booleans). So an enum cell is a badge with
 * its authored label and style, and a currency integer reads as an amount.
 *
 * The widget is resolved once per (entity type, property) and kept by the
 * resolver, because resolving walks a Map and can warn; doing that per cell
 * would repeat it for every row (RR-UD2A). Create one resolver per render
 * pass over a set of rows.
 */
export function createDenseCellResolver(): (
  value: unknown,
  propertyName: string,
  entityType: EntityType | undefined,
  tz: string
) => DenseCell | undefined {
  const widgets = new Map<
    EntityType,
    Map<string, { component: Component; hint: DenseRoutingHint }>
  >()

  return (value, propertyName, entityType, tz) => {
    // A cell without a known type or property stays on the plain-text path,
    // as an empty value does: widgets may draw a "no value" placeholder that
    // cells must not show.
    if (!entityType || isDenseEmpty(value)) return undefined
    const propertyDef = entityType.properties[propertyName]
    if (!propertyDef) return undefined
    let perType = widgets.get(entityType)
    if (!perType) {
      perType = new Map()
      widgets.set(entityType, perType)
    }
    let entry = perType.get(propertyName)
    if (!entry) {
      const hint = densePropertyRoutingHint(propertyDef, propertyName)
      entry = { component: defaultRegistry.resolveDense(hint), hint }
      perType.set(propertyName, entry)
    }
    return {
      component: entry.component,
      propertyName: entry.hint.propertyName,
      modelValue: entry.hint.preformatted
        ? formatCellValue(value, propertyName, entityType, tz)
        : value,
    }
  }
}
